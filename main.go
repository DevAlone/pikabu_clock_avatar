package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fogleman/gg"
	"gogsweb.2-47.ru/d3dev/pikago"
	"image/png"
	"io/ioutil"
	"math"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"
)

const ImageSize = 256
const LineWidth = ImageSize / 8
const (
	BackgroundR = 0.0
	BackgroundG = 0.0
	BackgroundB = 0.0
	BackgroundA = 0.0
)

type Line struct {
	StartPoint gg.Point
	EndPoint   gg.Point
}

var Config struct {
	Cookies     string
	ProxyAPIURL string
}

func getLineCoordinates(value int, maximumValue int, length float64) Line {
	value = value%maximumValue - maximumValue/4
	startPoint := gg.Point{
		X: ImageSize / 2,
		Y: ImageSize / 2,
	}
	endPoint := gg.Point{
		X: ImageSize/2 + math.Cos(float64(value)/float64(maximumValue)*(2.0*math.Pi))*length,
		Y: ImageSize/2 + math.Sin(float64(value)/float64(maximumValue)*(2.0*math.Pi))*length,
	}

	return Line{StartPoint: startPoint, EndPoint: endPoint}
}

func drawClockImage() *gg.Context {
	ctx := gg.NewContext(ImageSize, ImageSize)

	ctx.SetRGBA(BackgroundR, BackgroundG, BackgroundB, BackgroundA)
	ctx.Clear()
	tz, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}
	currentTime := time.Now().In(tz)

	hourLine := getLineCoordinates(currentTime.Hour(), 12, float64(ImageSize)/2.8)
	minuteLine := getLineCoordinates(currentTime.Minute(), 60, float64(ImageSize)/2.0)

	ctx.SetRGB(1, 0, 0)
	ctx.SetLineWidth(LineWidth)
	ctx.DrawLine(hourLine.StartPoint.X, hourLine.StartPoint.Y, hourLine.EndPoint.X, hourLine.EndPoint.Y)
	ctx.Stroke()

	ctx.DrawLine(minuteLine.StartPoint.X, minuteLine.StartPoint.Y, minuteLine.EndPoint.X, minuteLine.EndPoint.Y)
	ctx.SetRGB(0, 0, 1)
	ctx.Stroke()

	return ctx
}

func addFormField(w *multipart.Writer, key string, value []byte) error {
	fw, err := w.CreateFormField(key)
	if err != nil {
		return err
	}
	_, err = fw.Write(value)

	return err
}

func addFormPng(w *multipart.Writer, key string, fileName string, value []byte) error {
	h := make(textproto.MIMEHeader)

	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`, key, fileName))

	h.Set("Content-Type", "image/png")

	fw, err := w.CreatePart(h)

	_, err = fw.Write(value)

	return err
}

func UploadImage(ctx *gg.Context, pikagoClient *pikago.Client) error {
	b := bytes.Buffer{}
	w := multipart.NewWriter(&b)
	err := addFormField(w, "type", []byte("user_avatar"))
	if err != nil {
		return err
	}
	err = addFormField(w, "save", []byte("1"))
	if err != nil {
		return err
	}
	err = addFormField(w, "uid", []byte("0"))
	if err != nil {
		return err
	}
	imageBuffer := new(bytes.Buffer)
	err = png.Encode(imageBuffer, ctx.Image())
	if err != nil {
		return err
	}
	err = addFormPng(w, "0", "image.png", imageBuffer.Bytes())
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}

	url := "https://u.pikabu.ru/ajax/upload_file.php"

	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64; rv:60.0) Gecko/20100101 Firefox/60.0")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Origin", "https://pikabu.ru")
	req.Header.Set("Cookie", Config.Cookies)

	bodyBytes, res, err := pikagoClient.DoHttpRequest(req)
	if err != nil {
		return err
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("bad status %v, resp: %v", res.StatusCode, string(bodyBytes))
	}

	fmt.Printf("Response: %v\n", string(bodyBytes))

	return nil
}

func main() {
	configBytes, err := ioutil.ReadFile("config.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(configBytes, &Config)
	if err != nil {
		panic(err)
	}

	proxyProvider, err := pikago.GetProxyPyProxyProvider(Config.ProxyAPIURL, 10)
	if err != nil {
		panic(err)
	}
	requestsSender, err := pikago.NewClientProxyRequestsSender(proxyProvider)
	requestsSender.ChangeProxyOnNthBadTry = 1
	requestsSender.NumberOfRequestTries = 9999
	if err != nil {
		panic(err)
	}
	pikagoClient, err := pikago.NewClient(requestsSender)
	if err != nil {
		panic(err)
	}

	lastMinute := -1
	for true {
		if time.Now().Minute() != lastMinute {
			fmt.Println("drawing...")
			ctx := drawClockImage()

			err := UploadImage(ctx, pikagoClient)
			if err != nil {
				fmt.Println("Error happened")
				time.Sleep(5 * time.Second)
				panic(err)
			}
			lastMinute = time.Now().Minute()
		}

		time.Sleep(1 * time.Second)
	}
}
