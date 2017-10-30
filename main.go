package main

import (
    "github.com/golang/freetype"
    "image/jpeg"
    "os"
    "bufio"
    "image"
    "image/draw"
    "github.com/rwcarlsen/goexif/exif"
    "github.com/rwcarlsen/goexif/tiff"
    "golang.org/x/image/math/fixed"
    "github.com/golang/freetype/truetype"
    "github.com/lisijie/photowm/fonts"
    "flag"
    "path/filepath"
    "strings"
    "sync"
    "github.com/nfnt/resize"
    "fmt"
    "math"
    "net/http"
    "net/url"
    "io/ioutil"
    "github.com/bitly/go-simplejson"
    "errors"
    "runtime"
)

const DPI = 72.0
const LBS_KEY = "YRUBZ-4LV3R-RHSWK-WPZMU-IFSMK-22BVR"

// exif信息
type ExifInfo struct {
    Datetime    string  // 拍摄时间
    Longitude   float64 // 经度
    Latitude    float64 // 纬度
    Orientation int     // 拍摄方向, http://feihu.me/blog/2015/how-to-handle-image-orientation-on-iOS/
}

var (
    rlimit *RateLimit
    inFile = flag.String("file", "", "单张图片路径")
    inPath = flag.String("path", "", "处理指定目录下的所有图片")
    resizeWidth = flag.Uint("width", 3000, "指定图片最大宽度")
    fontSize = flag.Float64("fontsize", 0, "指定字体大小，0表示自动")
    outPath = flag.String("out", "", "输出目录")
    exts = []string{".jpg", ".jpeg"}
)

func main() {
    flag.Parse()

    rlimit = NewRateLimit(3) // 限速，每秒3次调用

    if *inFile == "" && *inPath == "" {
        flag.Usage()
        os.Exit(0)
    }

    if *outPath == "" {
        if *inFile != "" {
            *outPath = filepath.Join(filepath.Dir(*inFile), "out")
        } else {
            *outPath = filepath.Join(*inPath, "out")
        }
    }
    os.Mkdir(*outPath, 0755)

    if *inFile != "" {
        if err := handlePhoto(*inFile); err != nil {
            fmt.Println(err)
            os.Exit(1)
        }
        return
    }

    // 扫描目录
    photos := scanPath(*inPath)

    fmt.Printf("共找到 %d 张图片.\n", len(photos))
    if len(photos) < 1 {
        return
    }

    ch := make(chan string)
    go func(in chan <- string) {
        for _, v := range photos {
            in <- v
        }
        close(in)
    }(ch)

    var wg sync.WaitGroup
    for i := 0; i < runtime.NumCPU(); i++ {
        wg.Add(1)
        go func(out <- chan string, wg *sync.WaitGroup) {
            for v := range out {
                if err := handlePhoto(v); err != nil {
                    fmt.Printf("处理图片 %s ... ERR: %s\n", v, err)
                } else {
                    fmt.Printf("处理图片 %s ... OK\n", v)
                }
            }
            wg.Done()
        }(ch, &wg)
    }
    wg.Wait()
}

// 处理图片
func handlePhoto(filename string) error {
    fd, err := os.Open(filename)
    if err != nil {
        return err
    }
    img, err := jpeg.Decode(fd)
    if err != nil {
        return err
    }
    if *resizeWidth > 0 {
        img = resizePhoto(img, *resizeWidth)
    }
    exifInfo := getExifInfo(filename)
    if exifInfo != nil {
        if exifInfo.Orientation > 0 {
            img = fixOrientation(img, exifInfo.Orientation)
        }
        ss := make([]string, 0)
        ss = append(ss, exifInfo.Datetime)
        if exifInfo.Longitude > 0 && exifInfo.Latitude > 0 {
            for retry :=0 ; retry < 3; retry ++ { // 重试3次
                if s, err := geoAddr(exifInfo.Longitude, exifInfo.Latitude); err == nil {
                    ss = append(ss, s)
                    break
                } else {
                    return err
                }
            }
        }
        img = watermark(img, *fontSize, ss)
    }

    return saveToFile(img, filepath.Join(*outPath, filepath.Base(filename)))
}

// geo地址解析
func geoAddr(lo float64, la float64) (string, error) {
    rlimit.Take()
    u := "http://apis.map.qq.com/ws/geocoder/v1/?key=" + url.QueryEscape(LBS_KEY) + "&location=" + url.QueryEscape(fmt.Sprintf("%f,%f", la, lo))
    req, err := http.NewRequest("GET", u, nil)
    if err != nil {
        return "", err
    }
    req.Header.Set("User-Agent", ":Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.100 Safari/537.36")
    resp, err := http.DefaultClient.Do(req)
    if err != nil {
        return "", err
    }
    defer resp.Body.Close()
    body, err := ioutil.ReadAll(resp.Body)
    if err != nil {
        return "", err
    }
    js, err := simplejson.NewJson(body)
    if err != nil {
        return "", err
    }
    if js.Get("status").MustInt(-1) > 0 {
        return "", errors.New(js.Get("message").MustString(""))
    }
    addr := js.Get("result").Get("ad_info").Get("name").MustString("")
    return strings.Trim(addr, ","), nil
}

// 保存到文件
func saveToFile(img image.Image, filename string) error {
    f, err := os.Create(filename)
    if err != nil {
        return err
    }
    defer f.Close()
    b := bufio.NewWriter(f)
    err = jpeg.Encode(b, img, &jpeg.Options{Quality: 90})
    if err != nil {
        return err
    }
    err = b.Flush()
    if err != nil {
        return err
    }
    return nil
}

// 扫描目录，返回匹配的文件列表
func scanPath(root string) []string {
    result := make([]string, 0, 1000)
    filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            return nil
        }
        for _, v := range exts {
            if strings.HasSuffix(strings.ToLower(info.Name()), v) {
                result = append(result, path)
            }
        }
        return nil
    })
    return result
}

// 获取字体
func getFont(name string) (*truetype.Font, error) {
    data, err := fonts.Asset("resource/" + name)
    if err != nil {
        return nil, err
    }
    f, err := freetype.ParseFont(data)
    if err != nil {
        return nil, err
    }
    return f, nil
}

// 调整图片大小
func resizePhoto(img image.Image, size uint) image.Image {
    if img.Bounds().Dx() < int(size) && img.Bounds().Dy() < int(size) {
        return img
    }
    var w, h uint
    if img.Bounds().Dx() > img.Bounds().Dy() {
        w = size
    } else {
        h = size
    }
    return resize.Resize(w, h, img, resize.Lanczos3)
}

// 拍摄方向修复
func fixOrientation(img image.Image, o int) image.Image {
    w, h := img.Bounds().Dx(), img.Bounds().Dy()
    switch o {
    case 8: // 左竖
        dst := image.NewRGBA(image.Rect(0, 0, h, w))
        dw, dh := h, w
        for x := 0; x <= dw; x++ {
            for y := 0; y <= dh; y++ {
                dst.Set(x, y, img.At(w - y, x))
            }
        }
        return dst
    case 3: // 横下
        dst := image.NewRGBA(img.Bounds())
        for x := 0; x <= w; x++ {
            for y := 0; y <= h; y++ {
                dst.Set(x, y, img.At(w - x, h - y))
            }
        }
        return dst
    case 6: // 右竖
        dst := image.NewRGBA(image.Rect(0, 0, h, w))
        dw, dh := h, w
        for x := 0; x <= dw; x++ {
            for y := 0; y <= dh; y++ {
                dst.Set(x, y, img.At(y, h - x))
            }
        }
        return dst
    }
    return img
}

// 往图片增加水印文字
func watermark(src image.Image, fontSize float64, ss []string) image.Image {
    // 创建一个跟源图片一样大小的RGBA图片
    dst := image.NewRGBA(src.Bounds())
    // 将源图片拷贝到新图片中
    draw.Draw(dst, src.Bounds(), src, image.ZP, draw.Src)
    // 读取字体
    f, err := getFont("hwxihei.ttf")
    if err != nil {
        fmt.Println("getFont err: ", err.Error())
        return src
    }
    if fontSize < 1 {
        fontSize = 12 + math.Floor(math.Max(float64(src.Bounds().Dx()), float64(src.Bounds().Dy())) / 100)
    }
    c := freetype.NewContext()
    c.SetDPI(DPI)
    c.SetFont(f)
    c.SetFontSize(fontSize)
    c.SetClip(dst.Bounds()) // 设置可绘制区域
    c.SetDst(dst) // 设置绘制的目标图像
    for i := 0; i < len(ss); i ++ {
        // 计算字符串总宽度
        w := fixed.I(0)
        for _, r := range ss[i] {
            v := f.HMetric(c.PointToFixed(fontSize), f.Index(r))
            w = w + v.AdvanceWidth
        }
        // X, Y轴起始坐标, 绘制在右下角，偏移半个字大小
        x := dst.Bounds().Max.X - int(w >> 6) - int(fontSize / 2)
        y := dst.Bounds().Max.Y - int(c.PointToFixed(fontSize) >> 6) * (len(ss) - i - 1) - int(fontSize / 2)

        // 设置绘制点，使用黑色绘制一次
        c.SetSrc(image.Black)
        c.DrawString(ss[i], freetype.Pt(x, y))
        // 移动两个像素使用白色再绘制一次，实现阴影效果
        c.SetSrc(image.White)
        c.DrawString(ss[i], freetype.Pt(x - 1, y - 1))
    }
    return dst
}

// 从照片解析出拍摄日期和经纬度
func getExifInfo(fn string) *ExifInfo {
    ei := &ExifInfo{}
    f, err := os.Open(fn)
    if err != nil {
        return nil
    }
    defer f.Close()
    info, err := exif.Decode(f)
    if err != nil {
        return nil
    }
    // 获取拍摄方向
    if t, err := info.Get(exif.Orientation); err == nil {
        if v, err := t.Int(0); err == nil {
            ei.Orientation = v
        }
    }
    // 获取拍摄时间
    dt, err := info.DateTime()
    if err == nil {
        ei.Datetime = dt.Format("2006-01-02 15:04:05")
    }
    // 获取经纬度
    latitude, err := info.Get(exif.GPSLatitude)
    if err != nil {
        return ei
    }
    longitude, err := info.Get(exif.GPSLongitude)
    ei.Latitude, ei.Longitude = dms2dd(latitude), dms2dd(longitude)
    return ei
}

// 转换，将经纬度的度分秒格式转成小数形式
func dms2dd(t *tiff.Tag) float64 {
    var dms [3]float64
    for i := 0; i < 3; i++ {
        v1, v2, _ := t.Rat2(i)
        dms[i] = float64(v1 / v2)
    }
    dd := (dms[2] / 60 + dms[1]) / 60 + dms[0]
    return dd
}
