package bilinovel

import (
	"bilinovel-downloader/model"
	"bilinovel-downloader/utils"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	mapper "github.com/bestnite/font-mapper"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

//go:embed read.ttf
var readTTF []byte

//go:embed "MI LANTING.ttf"
var miLantingTTF []byte

type Bilinovel struct {
	fontMapper  *mapper.GlyphOutlineMapper
	textOnly    bool
	restyClient *utils.RestyClient
	debug       bool
}

func New() (*Bilinovel, error) {
	fontMapper, err := mapper.NewGlyphOutlineMapper(readTTF, miLantingTTF)
	if err != nil {
		return nil, fmt.Errorf("failed to create font mapper: %v", err)
	}
	restyClient := utils.NewRestyClient(10)
	return &Bilinovel{
		fontMapper:  fontMapper,
		textOnly:    false,
		restyClient: restyClient,
	}, nil
}

func (b *Bilinovel) SetTextOnly(textOnly bool) {
	b.textOnly = textOnly
}

func (b *Bilinovel) SetDebug(debug bool) {
	b.debug = debug
}

func (b *Bilinovel) GetExtraFiles() []model.ExtraFile {
	return nil
}

//go:embed style.css
var styleCSS []byte

func (b *Bilinovel) GetStyleCSS() string {
	return string(styleCSS)
}

func (b *Bilinovel) GetNovel(novelId int) (*model.Novel, error) {
	if b.debug {
		log.Printf("Getting novel %v\n", novelId)
	}
	novelUrl := fmt.Sprintf("https://www.bilinovel.com/novel/%v.html", novelId)
	resp, err := b.restyClient.R().Get(novelUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get novel info: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get novel info: %v", resp.Status())
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %v", err)
	}

	novel := &model.Novel{}

	novel.Title = strings.TrimSpace(doc.Find(".book-title").First().Text())
	novel.Description = strings.TrimSpace(doc.Find(".book-summary>content").First().Text())
	novel.Id = novelId

	doc.Find(".authorname>a").Each(func(i int, s *goquery.Selection) {
		novel.Authors = append(novel.Authors, strings.TrimSpace(s.Text()))
	})
	doc.Find(".illname>a").Each(func(i int, s *goquery.Selection) {
		novel.Authors = append(novel.Authors, strings.TrimSpace(s.Text()))
	})

	volumes, err := b.getAllVolumes(novelId)
	if err != nil {
		return nil, fmt.Errorf("failed to get novel volumes: %v", err)
	}
	novel.Volumes = volumes

	return novel, nil
}

func (b *Bilinovel) GetVolume(novelId int, volumeId int) (*model.Volume, error) {
	if b.debug {
		log.Printf("Getting volume %v of novel %v\n", volumeId, novelId)
	}
	novelUrl := fmt.Sprintf("https://www.bilinovel.com/novel/%v/catalog", novelId)
	resp, err := b.restyClient.R().Get(novelUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get novel info: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get novel info: %v", resp.Status())
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %v", err)
	}

	seriesIdx := 0
	doc.Find("a.volume-cover-img").Each(func(i int, s *goquery.Selection) {
		if s.AttrOr("href", "") == fmt.Sprintf("/novel/%v/vol_%v.html", novelId, volumeId) {
			seriesIdx = i + 1
		}
	})

	novelTitle := strings.TrimSpace(doc.Find(".book-title").First().Text())

	if seriesIdx == 0 {
		return nil, fmt.Errorf("volume not found: %v", volumeId)
	}

	volumeUrl := fmt.Sprintf("https://www.bilinovel.com/novel/%v/vol_%v.html", novelId, volumeId)
	resp, err = b.restyClient.R().Get(volumeUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get novel info: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get novel info: %v", resp.Status())
	}

	doc, err = goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %v", err)
	}

	volume := &model.Volume{}
	volume.NovelId = novelId
	volume.NovelTitle = novelTitle
	volume.Id = volumeId
	volume.SeriesIdx = seriesIdx
	volume.Title = strings.TrimSpace(doc.Find(".book-title").First().Text())
	volume.Description = strings.TrimSpace(doc.Find(".book-summary>content").First().Text())
	volume.Url = volumeUrl
	volume.Chapters = make([]*model.Chapter, 0)
	volume.CoverUrl = doc.Find(".book-cover").First().AttrOr("src", "")
	cover, err := b.getImg(volume.CoverUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get cover: %v", err)
	}
	volume.Cover = cover

	doc.Find(".authorname>a").Each(func(i int, s *goquery.Selection) {
		volume.Authors = append(volume.Authors, strings.TrimSpace(s.Text()))
	})
	doc.Find(".illname>a").Each(func(i int, s *goquery.Selection) {
		volume.Authors = append(volume.Authors, strings.TrimSpace(s.Text()))
	})
	doc.Find(".chapter-li.jsChapter").Each(func(i int, s *goquery.Selection) {
		volume.Chapters = append(volume.Chapters, &model.Chapter{
			Title: s.Find("a").Text(),
			Url:   fmt.Sprintf("https://www.bilinovel.com%v", s.Find("a").AttrOr("href", "")),
		})
	})

	idRegexp := regexp.MustCompile(`/novel/(\d+)/(\d+).html`)
	wg := sync.WaitGroup{}
	errChan := make(chan error, len(volume.Chapters))
	for i := range volume.Chapters {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			matches := idRegexp.FindStringSubmatch(volume.Chapters[i].Url)
			if len(matches) > 0 {
				chapterId, err := strconv.Atoi(matches[2])
				if err != nil {
					errChan <- fmt.Errorf("failed to convert chapter id: %v", err)
					return
				}
				chapter, err := b.GetChapter(novelId, volumeId, chapterId)
				if err != nil {
					errChan <- fmt.Errorf("failed to get chapter: %v", err)
					return
				}
				chapter.Id = chapterId
				volume.Chapters[i] = chapter
			} else {
				errChan <- fmt.Errorf("failed to get chapter id: %v", volume.Chapters[i].Url)
				return
			}
		}(i)
	}
	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			return nil, err
		}
	}
	return volume, nil
}

func (b *Bilinovel) getAllVolumes(novelId int) ([]*model.Volume, error) {
	if b.debug {
		log.Printf("Getting all volumes of novel %v\n", novelId)
	}
	catelogUrl := fmt.Sprintf("https://www.bilinovel.com/novel/%v/catalog", novelId)
	resp, err := b.restyClient.R().Get(catelogUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get catelog: %v", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("failed to get catelog: %v", resp.Status())
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(resp.Body()))
	if err != nil {
		return nil, fmt.Errorf("failed to parse html: %v", err)
	}

	volumeRegexp := regexp.MustCompile(fmt.Sprintf(`/novel/%v/vol_(\d+).html`, novelId))

	volumeIds := make([]string, 0)
	doc.Find("a.volume-cover-img").Each(func(i int, s *goquery.Selection) {
		link := s.AttrOr("href", "")
		matches := volumeRegexp.FindStringSubmatch(link)
		if len(matches) > 0 {
			volumeIds = append(volumeIds, matches[1])
		}
	})

	volumes := make([]*model.Volume, 0)
	for i, volumeIdStr := range volumeIds {
		volumeId, err := strconv.Atoi(volumeIdStr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert volume id: %v", err)
		}
		volume, err := b.GetVolume(novelId, volumeId)
		if err != nil {
			return nil, fmt.Errorf("failed to get volume info: %v", err)
		}
		volume.SeriesIdx = i
		volumes = append(volumes, volume)
	}

	return volumes, nil
}

func (b *Bilinovel) GetChapter(novelId int, volumeId int, chapterId int) (*model.Chapter, error) {
	if b.debug {
		log.Printf("Getting chapter %v of novel %v\n", chapterId, novelId)
	}
	page := 1
	chapter := &model.Chapter{
		Id:       chapterId,
		NovelId:  novelId,
		VolumeId: volumeId,
		Url:      fmt.Sprintf("https://www.bilinovel.com/novel/%v/%v.html", novelId, chapterId),
	}
	for {
		hasNext, err := b.getChapterByPage(chapter, page)
		if err != nil {
			return nil, fmt.Errorf("failed to download chapter: %w", err)
		}
		if !hasNext {
			break
		}
		page++
	}
	return chapter, nil
}

func (b *Bilinovel) getChapterByPage(chapter *model.Chapter, page int) (bool, error) {
	if b.debug {
		log.Printf("Getting chapter %v by page %v\n", chapter.Id, page)
	}

	Url := strings.TrimSuffix(chapter.Url, ".html") + fmt.Sprintf("_%v.html", page)

	hasNext := false
	headers := map[string]string{
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7",
		"Accept-Language": "zh-CN,zh;q=0.9,en-GB;q=0.8,en;q=0.7,zh-TW;q=0.6",
		"Cookie":          "night=1;",
	}
	resp, err := b.restyClient.R().SetHeaders(headers).Get(Url)
	if err != nil {
		return false, fmt.Errorf("failed to get chapter: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return false, fmt.Errorf("failed to get chapter: %v", resp.Status())
	}

	if strings.Contains(resp.String(), `<a onclick="window.location.href = ReadParams.url_next;">下一頁</a>`) {
		hasNext = true
	}

	html := resp.Body()
	// 解决乱序问题
	resortedHtml, err := ProcessContentWithChromedp(string(html))
	if err != nil {
		return false, fmt.Errorf("failed to process html: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resortedHtml))
	if err != nil {
		return false, fmt.Errorf("failed to parse html: %w", err)
	}

	if page == 1 {
		chapter.Title = doc.Find("#atitle").Text()
	}
	content := doc.Find("#acontent").First()
	content.Find(".cgo").Remove()
	content.Find("center").Remove()
	content.Find(".google-auto-placed").Remove()

	if strings.Contains(resp.String(), `font-family: "read"`) {
		html, err := content.Find("p").Last().Html()
		if err != nil {
			return false, fmt.Errorf("failed to get html: %v", err)
		}
		builder := strings.Builder{}
		for _, r := range html {
			_, newRune, ok := b.fontMapper.MappingRune(r)
			if ok {
				builder.WriteRune(newRune)
			}
		}
		content.Find("p").Last().SetHtml(builder.String())
	}

	if b.textOnly {
		content.Find("img").Remove()
	} else {
		content.Find("img").Each(func(i int, s *goquery.Selection) {
			imgUrl := s.AttrOr("data-src", "")
			if imgUrl == "" {
				imgUrl = s.AttrOr("src", "")
				if imgUrl == "" {
					return
				}
			}

			imageHash := sha256.Sum256([]byte(imgUrl))
			imageFilename := fmt.Sprintf("%x%s", string(imageHash[:]), path.Ext(imgUrl))
			s.SetAttr("src", imageFilename)
			s.SetAttr("alt", imgUrl)
			img, err := b.getImg(imgUrl)
			if err != nil {
				return
			}
			if chapter.Content == nil {
				chapter.Content = &model.ChaperContent{}
			}
			if chapter.Content.Images == nil {
				chapter.Content.Images = make(map[string][]byte)
			}
			chapter.Content.Images[imageFilename] = img
		})
	}

	htmlStr, err := content.Html()
	if err != nil {
		return false, fmt.Errorf("failed to get html: %v", err)
	}

	if chapter.Content == nil {
		chapter.Content = &model.ChaperContent{}
	}
	chapter.Content.Html += strings.TrimSpace(htmlStr)

	return hasNext, nil
}

func (b *Bilinovel) getImg(url string) ([]byte, error) {
	if b.debug {
		log.Printf("Getting img %v\n", url)
	}
	resp, err := b.restyClient.R().SetHeader("Referer", "https://www.bilinovel.com").Get(url)
	if err != nil {
		return nil, err
	}

	return resp.Body(), nil
}

func ProcessContentWithChromedp(htmlContent string) (string, error) {
	tempFile, err := os.CreateTemp("", "bilinovel-temp-*.html")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	_, err = tempFile.WriteString(htmlContent)
	if err != nil {
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()
	tempFilePath := tempFile.Name()

	// 创建chromedp选项
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("no-sandbox", true),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	// 设置超时
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var processedHTML string

	// 3. 执行chromedp任务并获取页面代码
	err = chromedp.Run(ctx,
		network.Enable(),

		// 等待JavaScript执行完成
		chromedp.ActionFunc(func(ctx context.Context) error {
			// 监听网络事件
			networkEventChan := make(chan bool, 1)
			requestID := ""
			chromedp.ListenTarget(ctx, func(ev interface{}) {
				switch ev := ev.(type) {
				case *network.EventRequestWillBeSent:
					if strings.Contains(ev.Request.URL, "chapterlog.js") {
						requestID = ev.RequestID.String()
					}
				case *network.EventLoadingFinished:
					if ev.RequestID.String() == requestID {
						networkEventChan <- true
					}
				}
			})

			go func() {
				select {
				case <-networkEventChan:
				case <-time.After(30 * time.Second):
					log.Println("Timeout waiting for external script")
				case <-ctx.Done():
					log.Println("Context cancelled")
				}
			}()
			return nil
		}),
		// 导航到本地文件
		chromedp.Navigate("file://"+filepath.ToSlash(tempFilePath)),
		// 等待页面加载完成
		chromedp.WaitVisible(`#acontent`, chromedp.ByID),
		// 获取页面的HTML代码
		chromedp.OuterHTML("html", &processedHTML, chromedp.ByQuery),
	)

	if err != nil {
		return "", fmt.Errorf("chromedp execution failed: %w", err)
	}

	return processedHTML, nil
}
