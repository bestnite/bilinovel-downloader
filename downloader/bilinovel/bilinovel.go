package bilinovel

import (
	"bilinovel-downloader/model"
	"bilinovel-downloader/utils"
	"bytes"
	"crypto/sha256"
	_ "embed"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/PuerkitoBio/goquery"
	mapper "github.com/bestnite/font-mapper"
	"github.com/playwright-community/playwright-go"
)

//go:embed read.ttf
var readTTF []byte

//go:embed "MI LANTING.ttf"
var miLantingTTF []byte

type Bilinovel struct {
	fontMapper  *mapper.GlyphOutlineMapper
	textOnly    bool
	restyClient *utils.RestyClient

	// 浏览器实例复用
	browser        playwright.Browser
	browserContext playwright.BrowserContext
	pages          map[string]playwright.Page
	concurrency    int
	concurrentChan chan any

	logger *slog.Logger
}

type BilinovelNewOption struct {
	Concurrency int
	Debug       bool
}

func New(option BilinovelNewOption) (*Bilinovel, error) {
	fontMapper, err := mapper.NewGlyphOutlineMapper(readTTF, miLantingTTF)
	if err != nil {
		return nil, fmt.Errorf("failed to create font mapper: %v", err)
	}
	restyClient := utils.NewRestyClient(50)

	var logLevel slog.Level
	if option.Debug {
		logLevel = slog.LevelDebug
	} else {
		logLevel = slog.LevelInfo
	}

	handlerOptions := &slog.HandlerOptions{
		Level: logLevel,
	}

	b := &Bilinovel{
		fontMapper:     fontMapper,
		textOnly:       false,
		restyClient:    restyClient,
		pages:          make(map[string]playwright.Page),
		concurrency:    option.Concurrency,
		concurrentChan: make(chan any, option.Concurrency),
		logger:         slog.New(slog.NewTextHandler(os.Stdout, handlerOptions)),
	}

	// 初始化浏览器实例
	err = b.initBrowser(option.Debug)
	if err != nil {
		return nil, fmt.Errorf("failed to init browser: %v", err)
	}

	return b, nil
}

func (b *Bilinovel) SetTextOnly(textOnly bool) {
	b.textOnly = textOnly
}

func (b *Bilinovel) GetExtraFiles() []model.ExtraFile {
	return nil
}

// initBrowser 初始化浏览器实例
func (b *Bilinovel) initBrowser(debug bool) error {
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("could not start playwright: %w", err)
	}

	b.browser, err = pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(!debug),
		Devtools: playwright.Bool(debug),
	})
	if err != nil {
		return fmt.Errorf("could not launch browser: %w", err)
	}

	b.browserContext, err = b.browser.NewContext()
	if err != nil {
		return fmt.Errorf("could not create browser context: %w", err)
	}

	b.logger.Info("Browser initialized successfully")
	return nil
}

// Close 清理资源
func (b *Bilinovel) Close() error {
	if b.browser != nil {
		if err := b.browser.Close(); err != nil {
			b.logger.Error("could not close browser", slog.Any("error", err))
		}
		b.browser = nil
		b.browserContext = nil
	}
	return nil
}

//go:embed style.css
var styleCSS []byte

func (b *Bilinovel) GetStyleCSS() string {
	return string(styleCSS)
}

func (b *Bilinovel) GetNovel(novelId int, skipChapterContent bool, skipVolumes []int) (*model.Novel, error) {
	b.logger.Info("Getting novel", slog.Int("novelId", novelId))

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

	volumes, err := b.getAllVolumes(novelId, skipChapterContent, skipVolumes)
	if err != nil {
		return nil, fmt.Errorf("failed to get novel volumes: %v", err)
	}
	novel.Volumes = volumes

	return novel, nil
}

func (b *Bilinovel) GetVolume(novelId int, volumeId int, skipChapterContent bool) (*model.Volume, error) {
	b.logger.Info("Getting volume of novel", slog.Int("volumeId", volumeId), slog.Int("novelId", novelId))

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

	if !skipChapterContent {
		for i := range volume.Chapters {
			matches := idRegexp.FindStringSubmatch(volume.Chapters[i].Url)
			if len(matches) > 0 {
				chapterId, err := strconv.Atoi(matches[2])
				if err != nil {
					return nil, fmt.Errorf("failed to convert chapter id: %v", err)
				}
				chapter, err := b.GetChapter(novelId, volumeId, chapterId)
				if err != nil {
					return nil, fmt.Errorf("failed to get chapter: %v", err)
				}
				chapter.Id = chapterId
				volume.Chapters[i] = chapter
			} else {
				return nil, fmt.Errorf("failed to get chapter id: %v", volume.Chapters[i].Url)
			}
		}
	}

	return volume, nil
}

func (b *Bilinovel) getAllVolumes(novelId int, skipChapterContent bool, skipVolumes []int) ([]*model.Volume, error) {
	b.logger.Info("Getting all volumes of novel", slog.Int("novelId", novelId))

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

	volumes := make([]*model.Volume, len(volumeIds))
	var wg sync.WaitGroup
	var mu sync.Mutex // 保护 volumes 写入的互斥锁

	for i, volumeIdStr := range volumeIds {
		wg.Add(1)
		b.concurrentChan <- struct{}{} // 获取一个并发槽

		go func(i int, volumeIdStr string) {
			defer wg.Done()
			defer func() { <-b.concurrentChan }() // 释放并发槽

			volumeId, err := strconv.Atoi(volumeIdStr)
			if err != nil {
				b.logger.Error("failed to convert volume id", slog.String("volumeIdStr", volumeIdStr), slog.Any("error", err))
				return
			}
			if slices.Contains(skipVolumes, volumeId) {
				return
			}
			volume, err := b.GetVolume(novelId, volumeId, skipChapterContent)
			if err != nil {
				b.logger.Error("failed to get volume info", slog.Int("novelId", novelId), slog.Int("volumeId", volumeId), slog.Any("error", err))
				return
			}
			volume.SeriesIdx = i

			// 关闭浏览器标签页
			pwPageKey := fmt.Sprintf("%v-%v", novelId, volumeId)
			if pwPage, ok := b.pages[pwPageKey]; ok {
				_ = pwPage.Close()
				delete(b.pages, pwPageKey)
			}

			mu.Lock()
			volumes[i] = volume
			mu.Unlock()
		}(i, volumeIdStr)
	}

	wg.Wait()

	// 过滤掉获取失败的 nil volume
	filteredVolumes := make([]*model.Volume, 0, len(volumes))
	for _, vol := range volumes {
		if vol != nil {
			filteredVolumes = append(filteredVolumes, vol)
		}
	}

	return filteredVolumes, nil
}

func (b *Bilinovel) GetChapter(novelId int, volumeId int, chapterId int) (*model.Chapter, error) {
	b.logger.Info("Getting chapter of novel", slog.Int("chapterId", chapterId), slog.Int("novelId", novelId))

	pageNum := 1
	chapter := &model.Chapter{
		Id:       chapterId,
		NovelId:  novelId,
		VolumeId: volumeId,
		Url:      fmt.Sprintf("https://www.bilinovel.com/novel/%v/%v.html", novelId, chapterId),
	}
	for {
		pwPageKey := fmt.Sprintf("%v-%v", novelId, volumeId)
		if _, ok := b.pages[pwPageKey]; !ok {
			pwPage, err := b.browserContext.NewPage()
			if err != nil {
				return nil, fmt.Errorf("failed to create browser page: %w", err)
			}
			b.pages[pwPageKey] = pwPage
		}
		hasNext, err := b.getChapterByPage(b.pages[pwPageKey], chapter, pageNum)
		if err != nil {
			return nil, fmt.Errorf("failed to download chapter: %w", err)
		}
		if !hasNext {
			break
		}
		pageNum++
	}
	return chapter, nil
}

var nextPageUrlRegexp = regexp.MustCompile(`url_next:\s?['"]([^'"]*?)['"]`)
var cleanNextPageUrlRegexp = regexp.MustCompile(`(_\d+)?\.html$`)

func (b *Bilinovel) getChapterByPage(pwPage playwright.Page, chapter *model.Chapter, pageNum int) (bool, error) {
	b.logger.Info("Getting chapter by page", slog.Int("chapter", chapter.Id), slog.Int("page", pageNum))

	Url := strings.TrimSuffix(chapter.Url, ".html") + fmt.Sprintf("_%v.html", pageNum)

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
	resortedHtml, err := b.processContentWithPlaywright(pwPage, string(html))
	if err != nil {
		return false, fmt.Errorf("failed to process html: %w", err)
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(resortedHtml))
	if err != nil {
		return false, fmt.Errorf("failed to parse html: %w", err)
	}

	// 判断章节是否有下一页
	n := nextPageUrlRegexp.FindStringSubmatch(resortedHtml)
	if len(n) != 2 {
		return false, fmt.Errorf("failed to determine wether there is a next page")
	}

	s := cleanNextPageUrlRegexp.ReplaceAllString(n[1], "")
	if strings.Contains(Url, s) {
		hasNext = true
	}

	if pageNum == 1 {
		chapter.Title = doc.Find("#atitle").Text()
	}
	content := doc.Find("#acontent").First()
	content.Find(".cgo").Remove()
	content.Find("center").Remove()
	content.Find(".google-auto-placed").Remove()

	if strings.Contains(resortedHtml, `font-family: "read"`) {
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
			s.RemoveAttr("class")
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

	doc.Find("*").Each(func(i int, s *goquery.Selection) {
		if len(s.Nodes) > 0 && len(s.Nodes[0].Attr) > 0 {
			// 遍历元素的所有属性
			for _, attr := range s.Nodes[0].Attr {
				// 3. 检查属性名是否以 "data-k" 开头，且属性值是否为空
				if strings.HasPrefix(attr.Key, "data-k") {
					// 4. 如果满足条件，就移除这个属性
					s.RemoveAttr(attr.Key)
				}
			}
		}
	})

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
	b.logger.Info("Getting img", slog.String("url", url))
	resp, err := b.restyClient.R().SetHeader("Referer", "https://www.bilinovel.com").Get(url)
	if err != nil {
		return nil, err
	}

	return resp.Body(), nil
}

// processContentWithPlaywright 使用复用的浏览器实例处理内容
func (b *Bilinovel) processContentWithPlaywright(page playwright.Page, htmlContent string) (string, error) {
	// 替换 window.location.replace，防止页面跳转
	htmlContent = strings.ReplaceAll(htmlContent, "window.location.replace", "console.log")

	tempPath := filepath.Join(os.TempDir(), "bilinovel-downloader")
	err := os.MkdirAll(tempPath, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create temp dir: %w", err)
	}
	tempFile, err := os.CreateTemp(tempPath, "temp-*.html")
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

	// // 屏蔽请求
	// googleAdsDomains := []string{
	// 	"adtrafficquality.google",
	// 	"doubleclick.net",
	// 	"googlesyndication.com",
	// 	"googletagmanager.com",
	// 	"hm.baidu.com",
	// 	"cloudflareinsights.com",
	// 	"fsdoa.js",                         // adblock 检测
	// 	"https://www.linovelib.com/novel/", // 阻止从本地文件跳转到在线页面
	// }
	// err = page.Route("**/*", func(route playwright.Route) {
	// 	for _, d := range googleAdsDomains {
	// 		if strings.Contains(route.Request().URL(), d) {
	// 			b.logger.Debug("blocking request", slog.String("url", route.Request().URL()))
	// 			err := route.Abort("aborted")
	// 			if err != nil {
	// 				b.logger.Debug("failed to block request", route.Request().URL(), err)
	// 			}
	// 			return
	// 		}
	// 	}
	// 	_ = route.Continue()
	// })
	// if err != nil {
	// 	return "", fmt.Errorf("failed to intercept requests: %w", err)
	// }

	_, err = page.ExpectResponse(func(url string) bool {
		return strings.Contains(url, "chapterlog.js")
	}, func() error {
		_, err = page.Goto("file://" + filepath.ToSlash(tempFilePath))
		if err != nil {
			return fmt.Errorf("could not navigate to file: %w", err)
		}
		return nil
	}, playwright.PageExpectResponseOptions{
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return "", fmt.Errorf("failed to wait for network request finish")
	}

	err = page.Locator("#acontent").WaitFor(playwright.LocatorWaitForOptions{
		State:   playwright.WaitForSelectorStateVisible,
		Timeout: playwright.Float(10000),
	})
	if err != nil {
		return "", fmt.Errorf("could not wait for #acontent: %w", err)
	}

	// 遍历所有 #acontent 的子元素, 通过 window.getComputedStyle().display 检测是否是 none, 如果是 none 则从页面删除这个元素
	result, err := page.Evaluate(`
		(function() {
			const acontent = document.getElementById('acontent');
			if (!acontent) {
				return 'acontent element not found';
			}
			
			let removedCount = 0;
			const elements = acontent.querySelectorAll('*');
			
			// 从后往前遍历，避免删除元素时影响索引
			for (let i = elements.length - 1; i >= 0; i--) {
				const element = elements[i];
				const computedStyle = window.getComputedStyle(element);
				
				if (computedStyle.display === 'none' || computedStyle.transform == 'matrix(0, 0, 0, 0, 0, 0)') {
					element.remove();
					removedCount++;
				}
			}
			
			return 'Removed ' + removedCount + ' hidden elements';
		})()
	`)

	if err != nil {
		return "", fmt.Errorf("failed to remove hidden elements: %w", err)
	}

	b.logger.Debug("Hidden elements removal result", slog.Any("count", result))

	processedHTML, err := page.Content()
	if err != nil {
		return "", fmt.Errorf("could not get page content: %w", err)
	}

	return processedHTML, nil
}
