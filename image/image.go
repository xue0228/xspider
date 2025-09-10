package image

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"

	"github.com/chai2010/tiff"
	"github.com/chai2010/webp"
)

// 图片格式魔术数字检测
var (
	pngHeader   = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	jpegHeader  = []byte{0xFF, 0xD8, 0xFF}
	gifHeader1  = []byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61} // GIF87a
	gifHeader2  = []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61} // GIF89a
	webpHeader  = []byte{0x52, 0x49, 0x46, 0x46}             // RIFF
	tiffHeader1 = []byte{0x49, 0x49, 0x2A, 0x00}             // II格式TIFF
	tiffHeader2 = []byte{0x4D, 0x4D, 0x00, 0x2A}             // MM格式TIFF
)

// DetectImageFormat 检测图片格式
func DetectImageFormat(data []byte) string {
	if len(data) < 8 {
		return ""
	}

	switch {
	case bytes.HasPrefix(data, pngHeader):
		return "png"
	case bytes.HasPrefix(data, jpegHeader):
		return "jpeg"
	case bytes.HasPrefix(data, gifHeader1) || bytes.HasPrefix(data, gifHeader2):
		return "gif"
	case bytes.HasPrefix(data, webpHeader) && len(data) >= 12 && bytes.Equal(data[8:12], []byte("WEBP")):
		return "webp"
	case bytes.HasPrefix(data, tiffHeader1) || bytes.HasPrefix(data, tiffHeader2):
		return "tiff"
	default:
		return ""
	}
}

// ReadImageFromBytes 从字节数据读取图片
func ReadImageFromBytes(data []byte) (image.Image, string, error) {
	// 检测图片格式
	format := DetectImageFormat(data)
	if format == "" {
		return nil, "", fmt.Errorf("无法识别图片格式")
	}

	// 使用相应的解码器解码
	switch format {
	case "png":
		img, err := png.Decode(bytes.NewReader(data))
		return img, format, err
	case "jpeg":
		img, err := jpeg.Decode(bytes.NewReader(data))
		return img, format, err
	case "gif":
		img, err := gif.Decode(bytes.NewReader(data))
		return img, format, err
	case "webp":
		img, err := webp.Decode(bytes.NewReader(data))
		return img, format, err
	case "tiff":
		img, err := tiff.Decode(bytes.NewReader(data))
		return img, format, err
	default:
		return nil, "", fmt.Errorf("不支持的图片格式: %s", format)
	}
}

// SaveImage 将图片数据保存为指定格式的文件
func SaveImage(img image.Image, path string, format string) error {
	// 确保保存目录存在
	if err := EnsureDir(path); err != nil {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("无法创建文件: %w", err)
	}
	defer file.Close()

	switch format {
	case "png":
		return png.Encode(file, img)
	case "jpeg", "jpg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
	case "gif":
		return gif.Encode(file, img, nil)
	case "webp":
		return webp.Encode(file, img, &webp.Options{Lossless: false, Quality: 100})
	case "tiff":
		return tiff.Encode(file, img, nil)
	default:
		return fmt.Errorf("不支持的图片格式: %s", format)
	}
}

// EnsureDir 递归创建目录（如果不存在）
func EnsureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "" {
		return nil // 没有目录需要创建
	}

	// 检查目录是否存在
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		// 递归创建目录，权限设置为0755
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("无法创建目录: %s: %w", dir, err)
		}
	} else if err != nil {
		return fmt.Errorf("检查目录时出错: %s: %w", dir, err)
	}
	return nil
}

// DrawImage 类似JavaScript中的drawImage函数
// 从源图像(src)中裁剪矩形区域(sx, sy)到(sx+sw, sy+sh)
// 绘制到目标图像(dst)的(dx, dy)位置，缩放为(dw, dh)大小
func DrawImage(dst *image.RGBA, src image.Image, sx, sy, sw, sh, dx, dy, dw, dh int) error {
	if dst == nil {
		return fmt.Errorf("目标图像不能为nil")
	}

	// 获取源图像边界
	srcBounds := src.Bounds()

	// 验证源图像区域
	if sx < srcBounds.Min.X || sy < srcBounds.Min.Y ||
		sx+sw > srcBounds.Max.X || sy+sh > srcBounds.Max.Y {
		return errors.New("source image region out of bounds")
	}

	// 验证目标图像区域
	dstBounds := dst.Bounds()
	if dx < dstBounds.Min.X || dy < dstBounds.Min.Y ||
		dx+dw > dstBounds.Max.X || dy+dh > dstBounds.Max.Y {
		return errors.New("destination image region out of bounds")
	}

	// 如果不需要缩放且源和目标大小相同，直接使用draw.Draw
	if sw == dw && sh == dh {
		// 提取源图像的子图像
		srcSubImg := src.(interface {
			SubImage(r image.Rectangle) image.Image
		}).SubImage(image.Rect(sx, sy, sx+sw, sy+sh))

		// 绘制到目标图像
		draw.Draw(dst, image.Rect(dx, dy, dx+dw, dy+dh), srcSubImg, image.Point{sx, sy}, draw.Src)
		return nil
	}

	// 处理缩放情况，使用双线性插值
	for y := 0; y < dh; y++ {
		// 计算在源图像中对应的y坐标（带小数部分）
		srcY := float64(sy) + float64(y)*float64(sh)/float64(dh)

		for x := 0; x < dw; x++ {
			// 计算在源图像中对应的x坐标（带小数部分）
			srcX := float64(sx) + float64(x)*float64(sw)/float64(dw)

			// 获取源图像中该点的颜色（双线性插值）
			c := bilinearInterpolate(src, srcX, srcY)

			// 设置目标图像中对应位置的颜色
			dstX := dx + x
			dstY := dy + y
			dst.SetRGBA(dstX, dstY, c)
		}
	}

	return nil
}

// bilinearInterpolate 使用双线性插值获取图像中某点的颜色
func bilinearInterpolate(img image.Image, x, y float64) color.RGBA {
	// 获取整数坐标
	x1, y1 := int(x), int(y)
	x2, y2 := x1+1, y1+1

	// 确保不超出图像边界
	bounds := img.Bounds()
	if x2 >= bounds.Max.X {
		x2 = bounds.Max.X - 1
	}
	if y2 >= bounds.Max.Y {
		y2 = bounds.Max.Y - 1
	}

	// 计算小数部分
	fx := x - float64(x1)
	fy := y - float64(y1)

	// 获取四个周围点的颜色
	c11 := getRGBA(img, x1, y1)
	c12 := getRGBA(img, x1, y2)
	c21 := getRGBA(img, x2, y1)
	c22 := getRGBA(img, x2, y2)

	// 双线性插值计算
	r := uint8(lerp(lerp(float64(c11.R), float64(c21.R), fx), lerp(float64(c12.R), float64(c22.R), fx), fy))
	g := uint8(lerp(lerp(float64(c11.G), float64(c21.G), fx), lerp(float64(c12.G), float64(c22.G), fx), fy))
	b := uint8(lerp(lerp(float64(c11.B), float64(c21.B), fx), lerp(float64(c12.B), float64(c22.B), fx), fy))
	a := uint8(lerp(lerp(float64(c11.A), float64(c21.A), fx), lerp(float64(c12.A), float64(c22.A), fx), fy))

	return color.RGBA{R: r, G: g, B: b, A: a}
}

// getRGBA 获取图像在(x,y)处的RGBA颜色
func getRGBA(img image.Image, x, y int) color.RGBA {
	r, g, b, a := img.At(x, y).RGBA()
	// 转换为8位颜色值
	return color.RGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

// lerp 线性插值
func lerp(a, b, t float64) float64 {
	return a + t*(b-a)
}
