package main

import (
 "bytes"
 "crypto/sha256"
 "encoding/base64"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"
 "time"
 "github.com/gorilla/websocket"
)

func TestViewerKeyExpiry(t *testing.T) {
 owner:=base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{'k'},24))
 portal:=base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{'p'},24))
 r:=&room{agent:&peer{conn:&websocket.Conn{}},created:time.Now().Add(-time.Hour)}
 r.hash=sha256.Sum256([]byte(owner))
 r.portalHash=sha256.Sum256([]byte(portal))
 r.portalExpiry=time.Now().Add(time.Minute)
 if validViewerKey(r,owner){t.Fatal("original link should expire")}
 if !validViewerKey(r,portal){t.Fatal("authorized portal viewer rejected")}
 r.portalExpiry=time.Now().Add(-time.Second)
 if validViewerKey(r,portal){t.Fatal("expired portal viewer allowed")}
}

func TestPortalAuthenticationAndDeviceWatch(t *testing.T) {
 id:=base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{'r'},12))
 key:=base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{'k'},24))
 s:=&relay{rooms:map[string]*room{
  id:{hash:sha256.Sum256([]byte(key)),name:"Test Laptop",agent:&peer{conn:&websocket.Conn{}},created:time.Now()},
 },portalUser:"hi5",portalPass:"test-portal-password-very-long"}
 unauthorized:=httptest.NewRecorder()
 s.portalDevices(unauthorized,httptest.NewRequest("GET","http://example.invalid/api/portal/devices",nil))
 if unauthorized.Code!=http.StatusUnauthorized{t.Fatalf("expected HTTP 401, got %d",unauthorized.Code)}
 req:=httptest.NewRequest("GET","http://example.invalid/api/portal/devices",nil)
 req.SetBasicAuth("hi5","test-portal-password-very-long")
 rec:=httptest.NewRecorder();s.portalDevices(rec,req)
 if rec.Code!=200{t.Fatalf("authenticated list HTTP %d",rec.Code)}
 var listed struct{Devices []deviceRecord `json:"devices"`}
 if err:=json.Unmarshal(rec.Body.Bytes(),&listed);err!=nil||len(listed.Devices)!=1||listed.Devices[0].Name!="Test Laptop"{
  t.Fatalf("unexpected device listing: %s / %v",rec.Body.String(),err)
 }
 if strings.Contains(rec.Body.String(),key)||strings.Contains(rec.Body.String(),"hash"){t.Fatal("private session credential leaked")}
 body:=strings.NewReader(`{"room":"`+id+`"}`)
 watch:=httptest.NewRequest("POST","https://jpeg.hi5central.com/api/portal/watch",body)
 watch.SetBasicAuth("hi5","test-portal-password-very-long")
 watch.Header.Set("Origin","https://jpeg.hi5central.com")
 watch.Header.Set("X-H5C-Portal","1")
 result:=httptest.NewRecorder();s.portalWatch(result,watch)
 if result.Code!=200{t.Fatalf("watch HTTP %d: %s",result.Code,result.Body.String())}
 var link struct{URL string `json:"url"`}
 if err:=json.Unmarshal(result.Body.Bytes(),&link);err!=nil{t.Fatal(err)}
 if !strings.HasPrefix(link.URL,"/?room="){t.Fatalf("unexpected watch link %q",link.URL)}
 request:=httptest.NewRequest("GET","https://jpeg.hi5central.com"+link.URL,nil)
 if _,err:=s.findRoom(request.URL.Query().Get("room"),request.URL.Query().Get("key"));err!=nil{t.Fatalf("portal capability not recognized: %v",err)}
 other:=httptest.NewRecorder()
 attack:=httptest.NewRequest("POST","https://jpeg.hi5central.com/api/portal/watch",strings.NewReader(`{"room":"`+id+`"}`))
 attack.SetBasicAuth("hi5","test-portal-password-very-long")
 attack.Header.Set("Origin","https://attacker.invalid")
 attack.Header.Set("X-H5C-Portal","1")
 s.portalWatch(other,attack)
 if other.Code!=http.StatusForbidden{t.Fatal("cross-origin watch should be blocked")}
}

func TestSanitizeDeviceName(t *testing.T){
 got:=deviceLabel("  Win-PC<script>alert(1)</script>  ")
 if got!="Win-PCscriptalert1script"{t.Fatalf("unexpected sanitized name: %q",got)}
}
