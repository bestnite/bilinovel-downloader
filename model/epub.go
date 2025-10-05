package model

import (
	"encoding/xml"
)

type ExtraFile struct {
	Data         []byte
	Path         string
	ManifestItem ManifestItem
}

type DublinCoreMetadata struct {
	XMLName xml.Name `xml:"metadata"`

	// 必需元素
	Titles      []DCTitle      `xml:"dc:title"`
	Identifiers []DCIdentifier `xml:"dc:identifier"`
	Languages   []DCLanguage   `xml:"dc:language"`

	// 可选元素
	Contributors []DCContributor `xml:"dc:contributor"`
	Coverages    []DCCoverage    `xml:"dc:coverage"`
	Creators     []DCCreator     `xml:"dc:creator"`
	Dates        []DCDate        `xml:"dc:date"`
	Descriptions []DCDescription `xml:"dc:description"`
	Formats      []DCFormat      `xml:"dc:format"`
	Publishers   []DCPublisher   `xml:"dc:publisher"`
	Relations    []DCRelation    `xml:"dc:relation"`
	Rights       []DCRights      `xml:"dc:rights"`
	Subjects     []DCSubject     `xml:"dc:subject"`
	Types        []DCType        `xml:"dc:type"`

	// EPUB3 扩展的 <meta> 元素
	Metas []DublinCoreMeta `xml:"meta"` // <meta> 用于扩展元数据
}

func (d *DublinCoreMetadata) Marshal() (string, error) {
	xmlBytes, err := xml.Marshal(d)
	if err != nil {
		return "", err
	}
	return string(xmlBytes), nil
}

// DCTitle 表示 <dc:title>
type DCTitle struct {
	Value string `xml:",chardata"`               // 标题内容
	ID    string `xml:"id,attr,omitempty"`       // 标题的唯一 ID
	Lang  string `xml:"xml:lang,attr,omitempty"` // 语言
}

// DCIdentifier 表示 <dc:identifier>
type DCIdentifier struct {
	Value  string `xml:",chardata"`                 // 标识符内容（如 UUID、ISBN）
	ID     string `xml:"id,attr,omitempty"`         // 标识符的唯一 ID
	Scheme string `xml:"opf:scheme,attr,omitempty"` // 标识符的方案（如 "uuid"）
}

// DCLanguage 表示 <dc:language>
type DCLanguage struct {
	Value string `xml:",chardata"` // 语言代码（如 "en"、"zh"）
}

// DCContributor 表示 <dc:contributor>
type DCContributor struct {
	Value  string `xml:",chardata"`                  // 贡献者名称
	ID     string `xml:"id,attr,omitempty"`          // 唯一 ID
	Role   string `xml:"opf:role,attr,omitempty"`    // 角色（如 "edt"、"ill"）
	FileAs string `xml:"opf:file-as,attr,omitempty"` // 规范化名称
	Lang   string `xml:"xml:lang,attr,omitempty"`    // 语言
}

// DCCoverage 表示 <dc:coverage>
type DCCoverage struct {
	Value string `xml:",chardata"`               // 地理或时间范围
	Lang  string `xml:"xml:lang,attr,omitempty"` // 语言
}

// DCCreator 表示 <dc:creator>
type DCCreator struct {
	Value  string `xml:",chardata"`                  // 创作者名称
	ID     string `xml:"id,attr,omitempty"`          // 唯一 ID
	Role   string `xml:"opf:role,attr,omitempty"`    // 角色（如 "aut"）
	FileAs string `xml:"opf:file-as,attr,omitempty"` // 规范化名称
	Lang   string `xml:"xml:lang,attr,omitempty"`    // 语言
}

// DCDate 表示 <dc:date>
type DCDate struct {
	Value string `xml:",chardata"`                // 日期（如 "2023-01-01"）
	Event string `xml:"opf:event,attr,omitempty"` // 事件类型（如 "publication"）
}

// DCDescription 表示 <dc:description>
type DCDescription struct {
	Value string `xml:",chardata"`               // 描述内容
	Lang  string `xml:"xml:lang,attr,omitempty"` // 语言
}

// DCFormat 表示 <dc:format>
type DCFormat struct {
	Value string `xml:",chardata"` // 格式（如 "EPUB 3.0"）
}

// DCPublisher 表示 <dc:publisher>
type DCPublisher struct {
	Value string `xml:",chardata"`               // 出版者名称
	Lang  string `xml:"xml:lang,attr,omitempty"` // 语言
}

// DCRelation 表示 <dc:relation>
type DCRelation struct {
	Value string `xml:",chardata"` // 相关资源标识符
}

// DCRights 表示 <dc:rights>
type DCRights struct {
	Value string `xml:",chardata"`               // 版权信息
	Lang  string `xml:"xml:lang,attr,omitempty"` // 语言
}

// DCSubject 表示 <dc:subject>
type DCSubject struct {
	Value string `xml:",chardata"`               // 主题或关键词
	Lang  string `xml:"xml:lang,attr,omitempty"` // 语言
}

// DCType 表示 <dc:type>
type DCType struct {
	Value string `xml:",chardata"` // 内容类型（如 "Text"、"Fiction"）
}

// DublinCoreMeta 表示 EPUB3 的 <meta> 扩展
type DublinCoreMeta struct {
	Name     string `xml:"name,attr,omitempty"`
	Content  string `xml:"content,attr,omitempty"`
	Value    string `xml:",chardata"`
	Property string `xml:"property,attr,omitempty"`
}

type Manifest struct {
	XMLName xml.Name       `xml:"manifest"`
	Items   []ManifestItem `xml:"item"`
}

func (m *Manifest) Marshal() (string, error) {
	xmlBytes, err := xml.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(xmlBytes), nil
}

type ManifestItem struct {
	ID         string `xml:"id,attr"`
	Link       string `xml:"href,attr"`
	Media      string `xml:"media-type,attr,omitempty"`
	Properties string `xml:"properties,attr,omitempty"`
}

type Spine struct {
	XMLName xml.Name    `xml:"spine"`
	Toc     string      `xml:"toc,attr,omitempty"`
	Items   []SpineItem `xml:"itemref"`
}

func (s *Spine) Marshal() (string, error) {
	xmlBytes, err := xml.Marshal(s)
	if err != nil {
		return "", err
	}
	return string(xmlBytes), nil
}

type SpineItem struct {
	IDref string `xml:"idref,attr"`
}

type Guide struct {
	XMLName xml.Name    `xml:"guide"`
	Items   []GuideItem `xml:"reference"`
}

func (g *Guide) Marshal() (string, error) {
	xmlBytes, err := xml.Marshal(g)
	if err != nil {
		return "", err
	}
	return string(xmlBytes), nil
}

type GuideItem struct {
	Title string `xml:"title,attr"`
	Type  string `xml:"type,attr"`
	Link  string `xml:"href,attr"`
}
