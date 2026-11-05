package main

import (
 "crypto/hmac"
 "crypto/rand"
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
 "sort"
 "strings"
 "sync"
 "time"
 "unicode"

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
 portalHash [32]byte
 portalExpiry time.Time
 name string
 agent,viewer *peer
 created time.Time
}
type relay struct{
 mu sync.Mutex
 rooms map[string]*room
 webDir,stunURL,turnURL,turnSecret string
 portalUser,portalPass string
}
type signal struct {Type string `json:"type"`;SDP string `json:"sdp,omitempty"`}
func validToken(v string,n int)bool{
 if len(v)>128{return false}
 decoded,err:=base64.RawURLEncoding.DecodeString(v)
 return err==nil&&len(decoded)==n
}
func validViewerKey(r *room,key string)bool{
 if r==nil||r.agent==nil||!validToken(key,24){return false}
 hash:=sha256.Sum256([]byte(key))
 original:=subtle.ConstantTimeCompare(r.hash[:],hash[:])==1 && (r.viewer!=nil || time.Since(r.created)<10*time.Minute)
 portal:=time.Now().Before(r.portalExpiry)&&subtle.ConstantTimeCompare(r.portalHash[:],hash[:])==1
 return original||portal
}
func(s *relay) findRoom(roomID,key string)(*room,error){
 if !validToken(roomID,12)||!validToken(key,24){return nil,errors.New("invalid link")}
 s.mu.Lock();defer s.mu.Unlock()
 r:=s.rooms[roomID]
 if !validViewerKey(r,key){return nil,errors.New("link not available")}
 return r,nil
}
func deviceLabel(raw string)string{
 raw=strings.Map(func(ch rune)rune{
  if unicode.IsLetter(ch)||unicode.IsDigit(ch)||ch==' '||ch=='.'||ch=='-'||ch=='_'{return ch}
  return -1
 },raw)
 raw=strings.TrimSpace(raw)
 chars:=[]rune(raw)
 if len(chars)>56{raw=string(chars[:56])}
 if raw==""{return "Windows computer"}
 return raw
}
func(s *relay) portalAuth(w http.ResponseWriter,r *http.Request)bool{
 w.Header().Set("Cache-Control","no-store")
 if len(s.portalPass)<16{http.NotFound(w,r);return false}
 user,pass,ok:=r.BasicAuth()
 a:=sha256.Sum256([]byte(user))
 b:=sha256.Sum256([]byte(s.portalUser))
 c:=sha256.Sum256([]byte(pass))
 d:=sha256.Sum256([]byte(s.portalPass))
 if !ok||subtle.ConstantTimeCompare(a[:],b[:])!=1||subtle.ConstantTimeCompare(c[:],d[:])!=1{
  w.Header().Set("WWW-Authenticate",`Basic realm="Hi5 JPEG Devices", charset="UTF-8"`)
  http.Error(w,"Portal login required",http.StatusUnauthorized)
  return false
 }
 return true
}
type deviceRecord struct{
 Name string `json:"name"`
 Room string `json:"room"`
 Viewing bool `json:"viewing"`
}
func(s *relay) portalDevices(w http.ResponseWriter,r *http.Request){
 if !s.portalAuth(w,r){return}
 if r.Method!="GET"{http.Error(w,"Method not allowed",http.StatusMethodNotAllowed);return}
 s.mu.Lock()
 devices:=make([]deviceRecord,0,len(s.rooms))
 for id,room:=range s.rooms{
  if room.agent!=nil{devices=append(devices,deviceRecord{Name:room.name,Room:id,Viewing:room.viewer!=nil})}
 }
 s.mu.Unlock()
 sort.Slice(devices,func(i,j int)bool{return devices[i].Name<devices[j].Name})
 w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store")
 _ = json.NewEncoder(w).Encode(map[string]any{"devices":devices})
}
func(s *relay) portalWatch(w http.ResponseWriter,r *http.Request){
 if !s.portalAuth(w,r){return}
 if r.Method!="POST"{http.Error(w,"Method not allowed",http.StatusMethodNotAllowed);return}
 origin:=r.Header.Get("Origin")
 if (origin!="https://"+r.Host&&origin!="http://"+r.Host)||r.Header.Get("X-H5C-Portal")!="1"{
  http.Error(w,"Forbidden origin",http.StatusForbidden);return
 }
 var request struct{Room string `json:"room"`}
 body:=json.NewDecoder(http.MaxBytesReader(w,r.Body,512))
 body.DisallowUnknownFields()
 if body.Decode(&request)!=nil||!validToken(request.Room,12){http.Error(w,"Invalid request",http.StatusBadRequest);return}
 bytes:=make([]byte,24)
 if _,err:=rand.Read(bytes);err!=nil{http.Error(w,"Token generation failed",http.StatusInternalServerError);return}
 key:=base64.RawURLEncoding.EncodeToString(bytes)
 s.mu.Lock()
 current:=s.rooms[request.Room]
 if current==nil||current.agent==nil||current.viewer!=nil{
  s.mu.Unlock();http.Error(w,"Device offline or already viewed",http.StatusConflict);return
 }
 current.portalHash=sha256.Sum256([]byte(key))
 current.portalExpiry=time.Now().Add(120*time.Second)
 s.mu.Unlock()
 w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store")
 q:=url.Values{};q.Set("room",request.Room);q.Set("key",key)
 _ = json.NewEncoder(w).Encode(map[string]any{"url":"/?"+q.Encode(),"expiresInSeconds":120})
}
func(s *relay) portalPage(w http.ResponseWriter,r *http.Request){
 if r.URL.Path!="/portal"{http.NotFound(w,r);return}
 if !s.portalAuth(w,r){return}
 w.Header().Set("Referrer-Policy","no-referrer")
 w.Header().Set("X-Content-Type-Options","nosniff")
 w.Header().Set("X-Frame-Options","DENY")
 w.Header().Set("Content-Security-Policy","default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; base-uri 'none'; frame-ancestors 'none'")
 http.ServeFile(w,r,strings.TrimSuffix(s.webDir,"/")+"/portal.html")
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
  current=&room{hash:sha256.Sum256([]byte(key)),name:deviceLabel(r.URL.Query().Get("device")),created:time.Now(),agent:p}
  s.rooms[roomID]=current
 } else{
  if current==nil||current.viewer!=nil||!validViewerKey(current,key){
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
  turnURL:os.Getenv("TURN_URL"),turnSecret:os.Getenv("TURN_SECRET"),
  portalUser:getenv("PORTAL_USER","hi5"),portalPass:os.Getenv("PORTAL_PASSWORD")}
 mux:=http.NewServeMux()
 mux.HandleFunc("/ws",s.ws)
 mux.HandleFunc("/api/ice",s.ice)
 mux.HandleFunc("/api/portal/devices",s.portalDevices)
 mux.HandleFunc("/api/portal/watch",s.portalWatch)
 mux.HandleFunc("/portal",s.portalPage)
 mux.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
  if r.URL.Path!="/"{http.NotFound(w,r);return}
  if r.URL.Query().Get("room")==""&&r.URL.Query().Get("key")==""{http.Redirect(w,r,"/portal",http.StatusFound);return}
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
