//go:build windows

package main

import (
 "errors"
 "image"
 "syscall"
 "unsafe"
)

const srccopy=0x00CC0020
const captureblt=0x40000000
const dibRGBColors=0

var (
 user32=syscall.NewLazyDLL("user32.dll")
 gdi32=syscall.NewLazyDLL("gdi32.dll")
 getDC=user32.NewProc("GetDC")
 releaseDC=user32.NewProc("ReleaseDC")
 getSystemMetrics=user32.NewProc("GetSystemMetrics")
 setDPIAware=user32.NewProc("SetProcessDPIAware")
 createCompatibleDC=gdi32.NewProc("CreateCompatibleDC")
 deleteDC=gdi32.NewProc("DeleteDC")
 createDIBSection=gdi32.NewProc("CreateDIBSection")
 selectObject=gdi32.NewProc("SelectObject")
 deleteObject=gdi32.NewProc("DeleteObject")
 bitBlt=gdi32.NewProc("BitBlt")
)

type bitmapInfoHeader struct {
 Size uint32
 Width, Height int32
 Planes, BitCount uint16
 Compression, SizeImage uint32
 XPelsPerMeter, YPelsPerMeter int32
 ClrUsed, ClrImportant uint32
}
type bitmapInfo struct {
 Header bitmapInfoHeader
 Colors [3]uint32
}
type capturer struct {
 screenDC,memoryDC,bitmap,oldBitmap uintptr
 sourceW,sourceH int
 pixels uintptr
}

func newCapturer() (*capturer,error) {
 setDPIAware.Call()
 w,_,_:=getSystemMetrics.Call(0)
 h,_,_:=getSystemMetrics.Call(1)
 if w==0||h==0||w>16384||h>16384 {return nil,errors.New("primary display unavailable")}
 c:=&capturer{sourceW:int(w),sourceH:int(h)}
 c.screenDC,_,_=getDC.Call(0)
 if c.screenDC==0 {return nil,errors.New("GetDC failed: interactive desktop required")}
 c.memoryDC,_,_=createCompatibleDC.Call(c.screenDC)
 if c.memoryDC==0 {c.close();return nil,errors.New("CreateCompatibleDC failed")}
 bmi:=bitmapInfo{Header:bitmapInfoHeader{
  Size:uint32(unsafe.Sizeof(bitmapInfoHeader{})),
  Width:int32(w),Height:-int32(h),Planes:1,BitCount:32,
 }}
 c.bitmap,_,_=createDIBSection.Call(c.screenDC,uintptr(unsafe.Pointer(&bmi)),dibRGBColors,uintptr(unsafe.Pointer(&c.pixels)),0,0)
 if c.bitmap==0||c.pixels==0 {c.close();return nil,errors.New("CreateDIBSection failed")}
 c.oldBitmap,_,_=selectObject.Call(c.memoryDC,c.bitmap)
 if c.oldBitmap==0 {c.close();return nil,errors.New("SelectObject failed")}
 return c,nil
}
func(c *capturer) close(){
 if c.memoryDC!=0&&c.oldBitmap!=0 {selectObject.Call(c.memoryDC,c.oldBitmap)}
 if c.bitmap!=0 {deleteObject.Call(c.bitmap)}
 if c.memoryDC!=0 {deleteDC.Call(c.memoryDC)}
 if c.screenDC!=0 {releaseDC.Call(0,c.screenDC)}
}
func(c *capturer) grab(maxWidth int)(*image.RGBA,error){
 ok,_,_:=bitBlt.Call(c.memoryDC,0,0,uintptr(c.sourceW),uintptr(c.sourceH),c.screenDC,0,0,srccopy|captureblt)
 if ok==0 {return nil,errors.New("BitBlt failed; screen may be locked")}
 w,h:=c.sourceW,c.sourceH
 if maxWidth>0&&w>maxWidth {h=(h*maxWidth+w/2)/w;w=maxWidth}
 if w<1||h<1{return nil,errors.New("invalid capture size")}
 out:=image.NewRGBA(image.Rect(0,0,w,h))
 raw:=unsafe.Slice((*byte)(unsafe.Pointer(c.pixels)),c.sourceW*c.sourceH*4)
 for y:=0;y<h;y++ {
  sy:=y*c.sourceH/h
  for x:=0;x<w;x++ {
   sx:=x*c.sourceW/w
   s:=(sy*c.sourceW+sx)*4
   d:=(y*w+x)*4
   out.Pix[d],out.Pix[d+1],out.Pix[d+2],out.Pix[d+3]=raw[s+2],raw[s+1],raw[s],255
  }
 }
 return out,nil
}
