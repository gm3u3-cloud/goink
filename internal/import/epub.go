package imp

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/html"
)

// container.xml 结构
type container struct {
	XMLName   xml.Name   `xml:"container"`
	Rootfiles []rootfile `xml:"rootfiles>rootfile"`
}

type rootfile struct {
	FullPath  string `xml:"full-path,attr"`
	MediaType string `xml:"media-type,attr"`
}

// OPF (content.opf) 结构
type opf struct {
	XMLName  xml.Name    `xml:"package"`
	Metadata opfMeta     `xml:"metadata"`
	Spine    opfSpine    `xml:"spine"`
	Manifest opfManifest `xml:"manifest"`
}

type opfMeta struct {
	Title string `xml:"title"`
}

type opfSpine struct {
	Items []spineItem `xml:"itemref"`
}

type spineItem struct {
	IDref string `xml:"idref,attr"`
}

type opfManifest struct {
	Items []manifestItem `xml:"item"`
}

type manifestItem struct {
	ID   string `xml:"id,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"media-type,attr"`
}

func parseEpub(filePath string, logger *slog.Logger) (*Result, error) {
	if logger == nil {
		logger = slog.Default()
	}
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, fmt.Errorf("Не удалось открыть файл epub: %w", err)
	}
	defer r.Close()

	// 1. 找到 container.xml
	var containerPath string
	for _, f := range r.File {
		if f.Name == "META-INF/container.xml" {
			containerPath = f.Name
			break
		}
	}
	if containerPath == "" {
		return nil, fmt.Errorf("В файле epub не найден META-INF/container.xml")
	}

	// 2. 解析 container.xml，获取 OPF 路径
	opfPath, err := parseContainerXML(r, containerPath)
	if err != nil {
		return nil, err
	}

	// 3. 解析 OPF，获取元数据和阅读顺序
	opfDir := path.Dir(opfPath)
	o, err := parseOPF(r, opfPath)
	if err != nil {
		return nil, err
	}

	// 4. 建立 ID → href 映射
	idToHref := make(map[string]string)
	for _, item := range o.Manifest.Items {
		idToHref[item.ID] = item.Href
	}

	// 5. 按 spine 顺序提取各章文本
	var chapters []Chapter
	var skippedChapters []SkippedChapter
	for _, spine := range o.Spine.Items {
		href := idToHref[spine.IDref]
		if href == "" {
			skippedChapters = append(skippedChapters, SkippedChapter{
				Title:  spine.IDref,
				Reason: "Для главы отсутствует файл",
			})
			continue
		}
		// 解析相对于 OPF 的路径
		resolvedPath, err := resolvePath(opfDir, href)
		if err != nil {
			logger.Warn("epub: skipping chapter", "href", href, "err", err)
			skippedChapters = append(skippedChapters, SkippedChapter{
				Title:  href,
				Reason: fmt.Sprintf("Не удалось разрешить путь файла: %v", err),
			})
			continue
		}
		text, err := extractHTMLText(r, resolvedPath)
		if err != nil {
			logger.Warn("epub: skipping chapter", "path", resolvedPath, "err", err)
			skippedChapters = append(skippedChapters, SkippedChapter{
				Title:  resolvedPath,
				Reason: fmt.Sprintf("Не удалось прочитать содержимое файла: %v", err),
			})
			continue
		}
		text = strings.TrimSpace(text)
		title := extractChapterTitle(text)
		if text == "" && title == "" {
			skippedChapters = append(skippedChapters, SkippedChapter{
				Title:  resolvedPath,
				Reason: "Заголовок и содержимое пусты",
			})
			continue
		}
		chapters = append(chapters, Chapter{
			Title:   title,
			Content: text,
		})
	}

	if len(chapters) == 0 {
		return nil, fmt.Errorf("В файле epub не удалось извлечь ни одной главы")
	}

	for i := range chapters {
		if chapters[i].Title == "" {
			chapters[i].Title = fmt.Sprintf("Глава %d", i+1)
		}
	}

	return &Result{
		Title:           strings.TrimSpace(o.Metadata.Title),
		Chapters:        chapters,
		SkippedChapters: skippedChapters,
	}, nil
}

func parseContainerXML(r *zip.ReadCloser, path string) (string, error) {
	f, err := r.Open(path)
	if err != nil {
		return "", fmt.Errorf("Не удалось прочитать container.xml: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("Не удалось прочитать container.xml: %w", err)
	}

	var c container
	if err := xml.Unmarshal(data, &c); err != nil {
		return "", fmt.Errorf("Не удалось разобрать container.xml: %w", err)
	}
	if len(c.Rootfiles) == 0 {
		return "", fmt.Errorf("В container.xml не найден rootfile")
	}
	// Prefer standard EPUB media-type; fall back to first rootfile.
	for _, rf := range c.Rootfiles {
		if rf.MediaType == "application/oebps-package+xml" {
			return rf.FullPath, nil
		}
	}
	return c.Rootfiles[0].FullPath, nil
}

func parseOPF(r *zip.ReadCloser, path string) (*opf, error) {
	f, err := r.Open(path)
	if err != nil {
		return nil, fmt.Errorf("Не удалось прочитать OPF-файл: %w", err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("Не удалось прочитать OPF-файл: %w", err)
	}

	var o opf
	if err := xml.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("Не удалось разобрать OPF-файл: %w", err)
	}
	return &o, nil
}

func extractHTMLText(r *zip.ReadCloser, path string) (string, error) {
	// EPUB 中路径可能存在编码差异，尝试多种大小写
	f, err := r.Open(path)
	if err != nil {
		// 大小写不敏感查找
		for _, zf := range r.File {
			if strings.EqualFold(zf.Name, path) {
				f, err = r.Open(zf.Name)
				break
			}
		}
		if err != nil {
			return "", err
		}
	}
	defer f.Close()

	doc, err := html.Parse(f)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	var extract func(*html.Node)
	extract = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "head":
				return
			case "br", "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "li", "tr", "blockquote":
				buf.WriteString("\n")
			}
		}
		if n.Type == html.TextNode {
			text := strings.TrimSpace(n.Data)
			if text != "" {
				buf.WriteString(text)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			extract(c)
		}
	}
	extract(doc)

	// 清理多余空行
	result := strings.TrimSpace(buf.String())
	lines := strings.Split(result, "\n")
	var clean []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			clean = append(clean, line)
		}
	}
	return strings.Join(clean, "\n"), nil
}

func resolvePath(dir, href string) (string, error) {
	// EPUB 内部路径全部使用正斜杠，必须用 path 包而非 filepath
	// zip.File.Open 只能打开 zip 内文件，天然防止路径穿越到 zip 外
	if decoded, err := url.PathUnescape(href); err == nil {
		href = decoded
	} else {
		href = strings.ReplaceAll(href, "%20", " ")
	}
	result := dir + "/" + href
	result = path.Clean(result)
	return result, nil
}

// extractChapterTitle 从 HTML 文本中尝试提取标题（h1-h6 的第一行）。
func extractChapterTitle(raw string) string {
	lines := strings.SplitN(raw, "\n", 2)
	if len(lines) > 0 {
		title := strings.TrimSpace(lines[0])
		// 限制标题长度
		runes := []rune(title)
		if len(runes) > 60 {
			title = string(runes[:60]) + "..."
		}
		return title
	}
	return ""
}
