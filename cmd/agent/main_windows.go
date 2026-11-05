//go:build windows

package main

import (
 "bytes"
 "context"
 "crypto/rand"
 "encoding/base64"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "image/jpeg"
 "log"
 "net"
 "net/http"
 "net/url"
 "os"
 "os/signal"
 "path/filepath"
 "strings"
 "sync"
 "time"

 "github.com/gorilla/websocket"
 "github.com/pion/webrtc/v4"
)

var defaultServer = "https://jpeg.hi5central.com"
var guiBuild = "0"

func appDir() string{
 base:=os.Getenv("LOCALAPPDATA")
 if base=="" {base=os.TempDir()}
 return filepath.Join(base,"Hi5Central","JPEGWebRTC")
}
func agentStatus(status string){
 dir:=appDir()
 if err:=os.MkdirAll(dir,0700);err==nil {
   _ = os.WriteFile(filepath.Join(dir,"status.txt"),[]byte(status+"\n"),0600)
 }
}
func agentLog() {
 if err:=os.MkdirAll(appDir(),0700);err!=nil{return}
 file,err:=os.OpenFile(filepath.Join(appDir(),"agent.log"),os.O_CREATE|os.O_WRONLY|os.O_APPEND,0600)
 if err==nil{log.SetOutput(file)}
}
type settings struct {FPS,Width,Quality int}
type message struct {Type string `json:"type"`; SDP string `json:"sdp,omitempty"`}
type wsClient struct {c *websocket.Conn;mu sync.Mutex}
func(c *wsClient) send(v any)error {c.mu.Lock();defer c.mu.Unlock();return c.c.WriteJSON(v)}

func randomToken(n int)(string,error){
 b:=make([]byte,n);if _,err:=rand.Read(b);err!=nil{return "",err}
 return base64.RawURLEncoding.EncodeToString(b),nil
}
func viewerURL(base *url.URL,room,key string)string{
 u:=*base;u.Path="/";v:=url.Values{};v.Set("room",room);v.Set("key",key);u.RawQuery=v.Encode();return u.String()
}
func iceServers(base *url.URL,room,key string)([]webrtc.ICEServer,error){
 u:=*base;u.Path="/api/ice";q:=url.Values{};q.Set("room",room);q.Set("key",key);u.RawQuery=q.Encode()
 ctx,cancel:=context.WithTimeout(context.Background(),8*time.Second);defer cancel()
 req,err:=http.NewRequestWithContext(ctx,"GET",u.String(),nil);if err!=nil{return nil,err}
 response,err:=http.DefaultClient.Do(req);if err!=nil{return nil,err};defer response.Body.Close()
 if response.StatusCode!=200{return nil,fmt.Errorf("ICE server returned HTTP %d",response.StatusCode)}
 var raw struct {ICEServers []struct{URLs []string `json:"urls"`;Username string `json:"username"`;Credential string `json:"credential"`} `json:"iceServers"`}
 if err:=json.NewDecoder(response.Body).Decode(&raw);err!=nil{return nil,err}
 result:=make([]webrtc.ICEServer,0,len(raw.ICEServers))
 for _,s:=range raw.ICEServers {result=append(result,webrtc.ICEServer{URLs:s.URLs,Username:s.Username,Credential:s.Credential})}
 return result,nil
}
func stream(dc *webrtc.DataChannel,stopped <-chan struct{},cfg settings){
 capture,err:=newCapturer();if err!=nil{log.Printf("No interactive desktop: %v",err);return}
 defer capture.close()
 log.Printf("Sharing %dx%d, width limit %d, JPEG %d, max FPS %d",capture.sourceW,capture.sourceH,cfg.Width,cfg.Quality,cfg.FPS)
 tick:=time.NewTicker(time.Second/time.Duration(cfg.FPS));defer tick.Stop()
 var seq uint32
 sent,skipped:=0,0
 lastStats:=time.Now()
 for{
  select{case <-stopped:return;case <-tick.C:}
  if dc.ReadyState()!=webrtc.DataChannelStateOpen{return}
  if dc.BufferedAmount()>384*1024 {skipped++;continue}
  img,err:=capture.grab(cfg.Width);if err!=nil{log.Printf("Capture error: %v",err);return}
  var buf bytes.Buffer
  if err:=jpeg.Encode(&buf,img,&jpeg.Options{Quality:cfg.Quality});err!=nil{log.Printf("JPEG error: %v",err);continue}
  frame:=buf.Bytes()
  if len(frame)>3*1024*1024||dc.BufferedAmount()+uint64(len(frame))>2*1024*1024 {skipped++;continue}
  seq++
  for _,packet:=range framePackets(seq,frame){if err:=dc.Send(packet);err!=nil{log.Printf("WebRTC send: %v",err);return}}
  sent++
  if time.Since(lastStats)>10*time.Second {
   log.Printf("10s: sent=%d skipped=%d last JPEG=%d KB buffered=%d KB",sent,skipped,len(frame)/1024,dc.BufferedAmount()/1024)
   sent,skipped=0,0;lastStats=time.Now()
  }
 }
}
func peerConnection(ice []webrtc.ICEServer,cfg settings)(*webrtc.PeerConnection,error){
 pc,err:=webrtc.NewPeerConnection(webrtc.Configuration{ICEServers:ice});if err!=nil{return nil,err}
 pc.OnConnectionStateChange(func(s webrtc.PeerConnectionState){
  log.Printf("WebRTC: %s",s.String())
  if s==webrtc.PeerConnectionStateFailed||s==webrtc.PeerConnectionStateDisconnected||s==webrtc.PeerConnectionStateClosed {
    agentStatus("ready")
  }
 })
 pc.OnDataChannel(func(dc *webrtc.DataChannel){
  if dc.Label()!="jpeg"{_ = dc.Close();return}
  stopped:=make(chan struct{});var once sync.Once
  stop:=func(){once.Do(func(){close(stopped)})}
  dc.OnClose(func(){stop();agentStatus("ready")})
  dc.OnOpen(func(){agentStatus("streaming");go stream(dc,stopped,cfg)})
 })
 return pc,nil
}
func run()error{
 var server,deviceName string
 machineName,_:=os.Hostname()
 cfg:=settings{}
 flag.StringVar(&server,"server",defaultServer,"HTTPS signalling server URL")
 flag.StringVar(&deviceName,"device",machineName,"Device name displayed on the private portal")
 flag.IntVar(&cfg.FPS,"fps",12,"target FPS 1-60")
 flag.IntVar(&cfg.Width,"width",1280,"maximum JPEG width; 0 for native")
 flag.IntVar(&cfg.Quality,"quality",75,"JPEG quality 30-95")
 flag.Parse()
 if strings.TrimSpace(server)==""{return errors.New("missing server URL")}
 if cfg.FPS<1||cfg.FPS>60||cfg.Width<0||cfg.Width>8192||cfg.Quality<30||cfg.Quality>95 {
  return errors.New("invalid fps, width or JPEG quality")
 }
 base,err:=url.Parse(strings.TrimSuffix(strings.TrimSpace(server),"/"));if err!=nil{return err}
 if base.Scheme!="https"{
  if base.Scheme!="http"{return errors.New("server URL must use HTTPS")}
  host:=base.Hostname();ip:=net.ParseIP(host)
  if host!="localhost"&&host!="127.0.0.1"&&(ip==nil||!ip.IsLoopback()){return errors.New("HTTP allowed only on loopback for local tests")}
 }
 if base.Host==""||(base.Path!=""&&base.Path!="/")||base.RawQuery!=""{
  return errors.New("use the root URL without a path/query")
 }
 room,err:=randomToken(12);if err!=nil{return err}
 key,err:=randomToken(24);if err!=nil{return err}
 target:=*base;if target.Scheme=="https"{target.Scheme="wss"}else{target.Scheme="ws"}
 target.Path="/ws";q:=url.Values{};q.Set("role","agent");q.Set("room",room);q.Set("key",key);q.Set("device",deviceName);target.RawQuery=q.Encode()
 conn,_,err:=websocket.DefaultDialer.Dial(target.String(),nil);if err!=nil{return fmt.Errorf("signalling connection: %w",err)}
 defer conn.Close();ws:=&wsClient{c:conn}
 defer agentStatus("offline")
 fmt.Println("\n=== Hi5Central JPEG screen share ===")
 fmt.Println("VIEW ONLY. This window is sharing-enabled. Close it to stop.")
 fmt.Println("Device: "+deviceName)
 fmt.Println("Open the secure device portal: "+base.String()+"/portal")
 fmt.Println("A viewer selects this device there to begin streaming.")
 fmt.Println("\nPrivate fallback viewing link:\n"+viewerURL(base,room,key)+"\n")
 fmt.Println("Only share links with an intended viewer.")
 ice,err:=iceServers(base,room,key);if err!=nil{return err}
 signals:=make(chan os.Signal,1);signal.Notify(signals,os.Interrupt);defer signal.Stop(signals)
 go func(){<-signals;_ = conn.Close()}()
 var active *webrtc.PeerConnection
 defer func(){if active!=nil{_ = active.Close()}}()
 for {
  _,raw,err:=conn.ReadMessage();if err!=nil{return fmt.Errorf("signalling disconnected: %w",err)}
  var msg message;if json.Unmarshal(raw,&msg)!=nil{continue}
  switch msg.Type{
  case "registered":agentStatus("ready");log.Println("Waiting for a browser viewer.")
  case "ready":log.Println("Browser connected.")
  case "peer-left":
   if active!=nil{_ = active.Close();active=nil};agentStatus("ready");log.Println("Viewer disconnected.")
  case "offer":
   if active!=nil{_ = active.Close();active=nil}
   pc,err:=peerConnection(ice,cfg);if err!=nil{return err};active=pc
   if err:=pc.SetRemoteDescription(webrtc.SessionDescription{Type:webrtc.SDPTypeOffer,SDP:msg.SDP});err!=nil{log.Printf("Bad offer: %v",err);continue}
   answer,err:=pc.CreateAnswer(nil);if err!=nil{log.Printf("CreateAnswer: %v",err);continue}
   gathered:=webrtc.GatheringCompletePromise(pc)
   if err:=pc.SetLocalDescription(answer);err!=nil{log.Printf("SetLocalDescription: %v",err);continue}
   select{case <-gathered:case <-time.After(20*time.Second):log.Println("ICE gathering deadline")}
   if local:=pc.LocalDescription();local!=nil {
    if err:=ws.send(message{Type:"answer",SDP:local.SDP});err!=nil{return err}
   }
  }
 }
}
func main(){
 log.SetFlags(log.Ldate|log.Ltime|log.Lmicroseconds)
 if guiBuild=="1"{agentLog()}
 agentStatus("connecting")
 if err:=run();err!=nil{
  agentStatus("offline")
  log.Printf("Stopped: %v",err)
  if guiBuild!="1"{fmt.Println("Press Enter to close...");fmt.Scanln()}
 }
}
