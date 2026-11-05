package main

import (
 "crypto/hmac"
 "crypto/sha1"
 "crypto/sha256"
 "crypto/subtle"
 "encoding/base64"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "log"
 "net/http"
 "net/url"
 "os"
 "strings"
 "sync"
 "time"

 "github.com/gorilla/websocket"
)

type peer struct {
 conn *websocket.Conn
 writeMu sync.Mutex
}
func(p *peer) send(v any)error{
 p.writeMu.Lock();defer p.writeMu.Unlock()
 _ = p.conn.SetWriteDeadline(time.Now().Add(5*time.Second))
 return p.conn.WriteJSON(v)
}
func(p *peer) ping()error{
 p.writeMu.Lock();defer p.writeMu.Unlock()
 _ = p.conn.SetWriteDeadline(time.Now().Add(5*time.Second))
 return p.conn.WriteMessage(websocket.PingMessage,nil)
}
type room struct{
 hash [32]byte
 agent,viewer *peer
 created time.Time
}
type relay struct{
 mu sync.Mutex
 rooms map[string]*room
 webDir,stunURL,turnURL,turnSecret string
}
type signal struct {Type string `json:"type"`;SDP string `json:"sdp,omitempty"`}
func validToken(v string,n int)bool{
 if len(v)>128{return false}
 decoded,err:=base64.RawURLEncoding.DecodeString(v)
 return err==nil&&len(decoded)==n
}
func(s *relay) findRoom(roomID,key string)(*room,error){
 if !validToken(roomID,12)||!validToken(key,24){return nil,errors.New("invalid link")}
 hash:=sha256.Sum256([]byte(key))
 s.mu.Lock();defer s.mu.Unlock()
 r:=s.rooms[roomID]
 if r==nil||r.agent==nil||subtle.ConstantTimeCompare(r.hash[:],hash[:])!=1{return nil,errors.New("link not available")}
 if r.viewer==nil&&time.Since(r.created)>10*time.Minute{return nil,errors.New("link expired")}
 return r,nil
}
func(s *relay) ice(w http.ResponseWriter,r *http.Request){
 if _,err:=s.findRoom(r.URL.Query().Get("room"),r.URL.Query().Get("key"));err!=nil{
  http.Error(w,"Invalid or expired session",http.StatusForbidden);return
 }
 type iceServer struct{
  URLs []string `json:"urls"`
  Username string `json:"username,omitempty"`
  Credential string `json:"credential,omitempty"`
 }
 servers:=[]iceServer{}
 if s.stunURL!=""{servers=append(servers,iceServer{URLs:[]string{s.stunURL}})}
 if s.turnURL!=""&&s.turnSecret!=""{
  username:=fmt.Sprintf("%d:%s",time.Now().Add(time.Hour).Unix(),r.URL.Query().Get("room"))
  mac:=hmac.New(sha1.New,[]byte(s.turnSecret));_,_=mac.Write([]byte(username))
  servers=append(servers,iceServer{URLs:[]string{s.turnURL},Username:username,Credential:base64.StdEncoding.EncodeToString(mac.Sum(nil))})
 }
 w.Header().Set("Cache-Control","no-store");w.Header().Set("Content-Type","application/json")
 _ = json.NewEncoder(w).Encode(map[string]any{"iceServers":servers})
}
func(s *relay) ws(w http.ResponseWriter,r *http.Request){
 role:=r.URL.Query().Get("role");roomID:=r.URL.Query().Get("room");key:=r.URL.Query().Get("key")
 if role!="agent"&&role!="viewer"{http.Error(w,"Invalid role",400);return}
 if !validToken(roomID,12)||!validToken(key,24){http.Error(w,"Invalid link",400);return}
 if role=="viewer"{
  if _,err:=s.findRoom(roomID,key);err!=nil{http.Error(w,"Invalid or expired link",403);return}
 }
 upgrader:=websocket.Upgrader{
  ReadBufferSize:4096,WriteBufferSize:4096,
  CheckOrigin:func(req *http.Request)bool{
   origin:=req.Header.Get("Origin")
   if origin==""{return true} // Native Windows app
   u,err:=url.Parse(origin)
   return err==nil&&u.Host==req.Host&&(u.Scheme=="https"||u.Scheme=="http")
  },
 }
 conn,err:=upgrader.Upgrade(w,r,nil);if err!=nil{return}
 p:=&peer{conn:conn};var other *peer
 s.mu.Lock()
 current:=s.rooms[roomID]
 if role=="agent"{
  if current!=nil&&current.agent!=nil{s.mu.Unlock();_ = conn.Close();return}
  if len(s.rooms)>=128{s.mu.Unlock();_ = conn.Close();return}
  current=&room{hash:sha256.Sum256([]byte(key)),created:time.Now(),agent:p}
  s.rooms[roomID]=current
 } else{
  hash:=sha256.Sum256([]byte(key))
  if current==nil||current.viewer!=nil||subtle.ConstantTimeCompare(current.hash[:],hash[:])!=1{
   s.mu.Unlock();_ = conn.Close();return
  }
  current.viewer=p;other=current.agent
 }
 s.mu.Unlock()
 if role=="agent"{
  _ = p.send(map[string]string{"type":"registered"})
 } else{
  _ = p.send(map[string]string{"type":"ready"})
  if other!=nil{_ = other.send(map[string]string{"type":"ready"})}
 }
 defer func(){
  s.mu.Lock()
  cur:=s.rooms[roomID]
  if cur!=nil{
   if role=="agent"&&cur.agent==p {other=cur.viewer;delete(s.rooms,roomID)}
   if role=="viewer"&&cur.viewer==p {cur.viewer=nil;other=cur.agent}
  }
  s.mu.Unlock()
  _ = conn.Close()
  if other!=nil{_ = other.send(map[string]string{"type":"peer-left"})}
 }()
 conn.SetReadLimit(100*1024)
 _ = conn.SetReadDeadline(time.Now().Add(75*time.Second))
 conn.SetPongHandler(func(string)error{return conn.SetReadDeadline(time.Now().Add(75*time.Second))})
 done:=make(chan struct{})
 go func(){
  ticker:=time.NewTicker(25*time.Second);defer ticker.Stop()
  for{
   select{
   case <-done:return
   case <-ticker.C:if p.ping()!=nil{_ = conn.Close();return}
   }
  }
 }()
 defer close(done)
 for{
  mt,data,err:=conn.ReadMessage();if err!=nil{return}
  if mt!=websocket.TextMessage{continue}
  var msg signal
  if json.Unmarshal(data,&msg)!=nil{continue}
  if msg.Type!="offer"&&msg.Type!="answer"{continue}
  if len(msg.SDP)==0||len(msg.SDP)>90*1024{continue}
  s.mu.Lock()
  cur:=s.rooms[roomID];var dest *peer
  if cur!=nil{
   if role=="agent"&&cur.agent==p{dest=cur.viewer}
   if role=="viewer"&&cur.viewer==p{dest=cur.agent}
  }
  s.mu.Unlock()
  if dest!=nil{if err:=dest.send(msg);err!=nil{log.Printf("Signal delivery failed: %v",err)}}
 }
}
func getenv(name,fallback string)string{
 if value:=os.Getenv(name);value!=""{return value}
 return fallback
}
func main(){
 var listen,webDir string
 flag.StringVar(&listen,"listen",getenv("JPEG_LISTEN","127.0.0.1:8787"),"HTTP bind address (use reverse proxy for HTTPS)")
 flag.StringVar(&webDir,"web-dir",getenv("JPEG_WEB_DIR","./web"),"viewer folder")
 flag.Parse()
 s:=&relay{rooms:map[string]*room{},webDir:webDir,
  stunURL:getenv("STUN_URL","stun:stun.l.google.com:19302"),
  turnURL:os.Getenv("TURN_URL"),turnSecret:os.Getenv("TURN_SECRET")}
 mux:=http.NewServeMux()
 mux.HandleFunc("/ws",s.ws)
 mux.HandleFunc("/api/ice",s.ice)
 mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/"{http.NotFound(w,r);return}
  w.Header().Set("Referrer-Policy","no-referrer")
  w.Header().Set("X-Content-Type-Options","nosniff")
  w.Header().Set("Content-Security-Policy","default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self' wss: ws:; img-src 'self' blob:; base-uri 'none'; frame-ancestors 'none'")
  w.Header().Set("Cache-Control","no-store")
  http.ServeFile(w,r,strings.TrimSuffix(s.webDir,"/")+"/index.html")
 })
 server:=&http.Server{Addr:listen,Handler:mux,ReadHeaderTimeout:5*time.Second,IdleTimeout:100*time.Second}
 log.Printf("JPEG WebRTC signalling on %s (use HTTPS in production)",listen)
 log.Fatal(server.ListenAndServe())
}
