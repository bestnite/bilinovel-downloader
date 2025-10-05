package epub

import (
	"archive/zip"
	"bilinovel-downloader/model"
	"bilinovel-downloader/template"
	"bilinovel-downloader/utils"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

func PackVolumeToEpub(volume *model.Volume, outputPath string, styleCSS string, extraFiles []model.ExtraFile) error {
	outputPath = filepath.Join(outputPath, utils.CleanDirName(volume.Title))
	_, err := os.Stat(outputPath)
	if err != nil {
		if os.IsNotExist(err) {
			err = os.MkdirAll(outputPath, 0755)
			if err != nil {
				return fmt.Errorf("failed to create output directory: %v", err)
			}
		} else {
			return fmt.Errorf("failed to get output directory: %v", err)
		}
	} else {
		err = os.RemoveAll(outputPath)
		if err != nil {
			return fmt.Errorf("failed to remove output directory: %v", err)
		}
		err = os.MkdirAll(outputPath, 0755)
		if err != nil {
			return fmt.Errorf("failed to create output directory: %v", err)
		}
	}

	// 将文字写入 OEBPS/Text/chapter-%03v.xhtml
	// 将图片写入 OEBPS/Images/chapter-%03v/
	for i, chapter := range volume.Chapters {
		imageNames := make([]string, 0)
		for imgName, imgData := range chapter.Content.Images {
			imageNames = append(imageNames, imgName)
			imgPath := filepath.Join(outputPath, fmt.Sprintf("OEBPS/Images/chapter-%03v/%s", i, imgName))
			err := os.MkdirAll(filepath.Dir(imgPath), 0755)
			if err != nil {
				return fmt.Errorf("failed to create image directory: %v", err)
			}
			err = os.WriteFile(imgPath, imgData, 0644)
			if err != nil {
				return fmt.Errorf("failed to write image: %v", err)
			}
		}
		chapterPath := filepath.Join(outputPath, fmt.Sprintf("OEBPS/Text/chapter-%03v.xhtml", i))
		err = os.MkdirAll(filepath.Dir(chapterPath), 0755)
		if err != nil {
			return fmt.Errorf("failed to create chapter directory: %v", err)
		}
		file, err := os.Create(chapterPath)
		if err != nil {
			return fmt.Errorf("failed to create chapter file: %v", err)
		}
		defer file.Close()
		text := chapter.Content.Html
		for _, imgName := range imageNames {
			text = strings.ReplaceAll(text, imgName, fmt.Sprintf("../Images/chapter-%03v/%s", i, imgName))
		}
		err = template.ContentXHTML(chapter.Title, text).Render(context.Background(), file)
		if err != nil {
			return fmt.Errorf("failed to write chapter: %v", err)
		}
	}

	// 将 Cover 写入
	coverPath := filepath.Join(outputPath, fmt.Sprintf("cover%s", filepath.Ext(volume.CoverUrl)))
	err = os.WriteFile(coverPath, volume.Cover, 0644)
	if err != nil {
		return fmt.Errorf("failed to write cover: %v", err)
	}

	// 将 CoverXHTML 写入 OEBPS/Text/cover.xhtml
	coverXHTMLPath := filepath.Join(outputPath, "OEBPS/Text/cover.xhtml")
	file, err := os.Create(coverXHTMLPath)
	if err != nil {
		return fmt.Errorf("failed to create cover XHTML file: %v", err)
	}
	defer file.Close()
	err = template.CoverXHTML(fmt.Sprintf("../../%s", filepath.Base(coverPath))).Render(context.Background(), file)
	if err != nil {
		return fmt.Errorf("failed to render cover XHTML: %v", err)
	}

	// OEBPS/Text/contents.xhtml 目录
	contentsXHTMLPath := filepath.Join(outputPath, "OEBPS/Text/contents.xhtml")
	file, err = os.Create(contentsXHTMLPath)
	if err != nil {
		return fmt.Errorf("failed to create contents XHTML file: %v", err)
	}
	defer file.Close()
	contents := strings.Builder{}
	contents.WriteString(`<nav epub:type="toc" id="toc">`)
	contents.WriteString(`<ol>`)
	for i, chapter := range volume.Chapters {
		contents.WriteString(fmt.Sprintf(`<li><a href="chapter-%03v.xhtml">%s</a></li>`, i, chapter.Title))
	}
	contents.WriteString(`</ol>`)
	contents.WriteString(`</nav>`)
	err = template.ContentXHTML("目录", contents.String()).Render(context.Background(), file)
	if err != nil {
		return fmt.Errorf("failed to render contents XHTML: %v", err)
	}

	// ContainerXML
	containerPath := filepath.Join(outputPath, "META-INF/container.xml")
	err = os.MkdirAll(filepath.Dir(containerPath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create container directory: %v", err)
	}
	file, err = os.Create(containerPath)
	if err != nil {
		return fmt.Errorf("failed to create container file: %v", err)
	}
	defer file.Close()
	err = template.ContainerXML().Render(context.Background(), file)
	if err != nil {
		return fmt.Errorf("failed to render container: %v", err)
	}

	// ContentOPF
	u := uuid.New()
	err = CreateContentOPF(outputPath, u.String(), volume, extraFiles)
	if err != nil {
		return fmt.Errorf("failed to create content OPF: %v", err)
	}

	// 写入 CSS
	cssPath := filepath.Join(outputPath, "style.css")
	err = os.WriteFile(cssPath, []byte(styleCSS), 0644)
	if err != nil {
		return fmt.Errorf("failed to write CSS: %v", err)
	}

	// 写入 extraFiles
	for _, file := range extraFiles {
		extraFilePath := filepath.Join(outputPath, file.Path)
		err = os.WriteFile(extraFilePath, file.Data, 0644)
		if err != nil {
			return fmt.Errorf("failed to write extra file: %v", err)
		}
	}

	// 打包成 epub 文件
	err = PackEpub(outputPath)
	if err != nil {
		return fmt.Errorf("failed to pack epub: %v", err)
	}
	return nil
}

func CreateContentOPF(outputPath string, uuid string, volume *model.Volume, extraFiles []model.ExtraFile) error {
	creators := make([]model.DCCreator, 0)
	for _, author := range volume.Authors {
		creators = append(creators, model.DCCreator{
			Value: author,
		})
	}
	dc := &model.DublinCoreMetadata{
		Titles: []model.DCTitle{
			{
				Value: volume.Title,
			},
		},
		Identifiers: []model.DCIdentifier{
			{
				Value: fmt.Sprintf("urn:uuid:%s", uuid),
				ID:    "book-id",
				// Scheme: "UUID",
			},
		},
		Languages: []model.DCLanguage{
			{
				Value: "zh-CN",
			},
		},
		Descriptions: []model.DCDescription{
			{
				Value: volume.Description,
			},
		},
		Creators: creators,
		Metas: []model.DublinCoreMeta{
			{
				Name:    "cover",
				Content: "cover",
			},
			{
				Property: "dcterms:modified",
				Value:    time.Now().UTC().Format("2006-01-02T15:04:05Z"),
			},
			{
				Name:    "calibre:series",
				Content: volume.NovelTitle,
			},
			{
				Name:    "calibre:series_index",
				Content: strconv.Itoa(volume.SeriesIdx),
			},
		},
	}
	manifest := &model.Manifest{
		Items: make([]model.ManifestItem, 0),
	}
	manifest.Items = append(manifest.Items, model.ManifestItem{
		ID:    "cover.xhtml",
		Link:  "OEBPS/Text/cover.xhtml",
		Media: "application/xhtml+xml",
	})
	manifest.Items = append(manifest.Items, model.ManifestItem{
		ID:         "contents.xhtml",
		Link:       "OEBPS/Text/contents.xhtml",
		Media:      "application/xhtml+xml",
		Properties: "nav",
	})
	manifest.Items = append(manifest.Items, model.ManifestItem{
		ID:         "cover",
		Link:       fmt.Sprintf("cover%s", filepath.Ext(volume.CoverUrl)),
		Media:      fmt.Sprintf("image/%s", strings.ReplaceAll(strings.TrimPrefix(filepath.Ext(volume.CoverUrl), "."), "jpg", "jpeg")),
		Properties: "cover-image",
	})
	for i, chapter := range volume.Chapters {
		manifest.Items = append(manifest.Items, model.ManifestItem{
			ID:    fmt.Sprintf("chapter-%03v.xhtml", i),
			Link:  fmt.Sprintf("OEBPS/Text/chapter-%03v.xhtml", i),
			Media: "application/xhtml+xml",
		})
		for filename := range chapter.Content.Images {
			item := model.ManifestItem{
				ID:    fmt.Sprintf("chapter-%03v-%s", i, filepath.Base(filename)),
				Link:  fmt.Sprintf("OEBPS/Images/chapter-%03v/%s", i, filepath.Base(filename)),
				Media: fmt.Sprintf("image/%s", strings.ReplaceAll(strings.TrimPrefix(filepath.Ext(filename), "."), "jpg", "jpeg")),
			}
			manifest.Items = append(manifest.Items, item)
		}
	}
	manifest.Items = append(manifest.Items, model.ManifestItem{
		ID:    "style",
		Link:  "style.css",
		Media: "text/css",
	})
	// ExtraFiles
	for _, file := range extraFiles {
		manifest.Items = append(manifest.Items, file.ManifestItem)
	}

	spine := &model.Spine{
		Items: make([]model.SpineItem, 0),
	}
	for _, item := range manifest.Items {
		if filepath.Ext(item.Link) == ".xhtml" {
			spine.Items = append(spine.Items, model.SpineItem{
				IDref: item.ID,
			})
		}
	}
	contentOPFPath := filepath.Join(outputPath, "content.opf")
	err := os.MkdirAll(path.Dir(contentOPFPath), 0755)
	if err != nil {
		return fmt.Errorf("failed to create content directory: %v", err)
	}
	file, err := os.Create(contentOPFPath)
	if err != nil {
		return fmt.Errorf("failed to create content file: %v", err)
	}
	err = template.ContentOPF("book-id", dc, manifest, spine, nil).Render(context.Background(), file)
	if err != nil {
		return fmt.Errorf("failed to render content: %v", err)
	}
	return nil
}

func PackEpub(dirPath string) error {
	savePath := strings.TrimSuffix(dirPath, string(filepath.Separator)) + ".epub"
	zipFile, err := os.Create(savePath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	err = addStringToZip(zipWriter, "mimetype", "application/epub+zip", zip.Store)
	if err != nil {
		return err
	}

	err = addDirContentToZip(zipWriter, dirPath, zip.Deflate)
	if err != nil {
		return err
	}

	return nil
}

func addStringToZip(zipWriter *zip.Writer, relPath, content string, method uint16) error {
	header := &zip.FileHeader{
		Name:   relPath,
		Method: method,
	}
	writer, err := zipWriter.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = writer.Write([]byte(content))
	return err
}

func addDirContentToZip(zipWriter *zip.Writer, dirPath string, method uint16) error {
	return filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
		if filepath.Base(filePath) == "volume.json" {
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(dirPath, filePath)
		if err != nil {
			return err
		}

		relPath = filepath.ToSlash(relPath)

		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath
		header.Method = method

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}

		_, err = io.Copy(writer, file)
		return err
	})
}
