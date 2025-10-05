package downloader

import "bilinovel-downloader/model"

type Downloader interface {
	GetNovel(novelId int, skipChapterContent bool, skipVolumes []int) (*model.Novel, error)
	GetVolume(novelId int, volumeId int, skipChapterContent bool) (*model.Volume, error)
	GetChapter(novelId int, volumeId int, chapterId int) (*model.Chapter, error)
	GetStyleCSS() string
	GetExtraFiles() []model.ExtraFile
	Close() error
}
