# Bilinovel Downloader

这是一个用于从 Bilinovel 下载和生成轻小说 EPUB 电子书的工具。
生成的 EPUB 文件完全符合 EPUB 标准，可以在 Calibre 检查中无错误通过。

## 使用示例

1. 下载整本 `https://www.bilinovel.com/novel/2388.html`

   ```bash
   bilinovel-downloader download -n 2388
   ```

2. 下载单卷 `https://www.bilinovel.com/novel/2388/vol_84522.html`

   ```bash
   bilinovel-downloader download -n 2388 -v 84522
   ```

3. 对自动生成的 epub 格式不满意可以自行修改后使用命令打包

   ```bash
   bilinovel-downloader pack -d <目录路径>
   ```

## 算法分析

目前程序使用 playwright 进行爬取来规避 bilinovel 的反爬（诱饵段落和段落重排）策略。  
但是依然对 bilinovel 的算法进行了简单的分析，具体可以参考[代码](./test/no_playwright_method_test.go)，这个代码目前是可行的，但如果 bilinovel 频繁更改初始化种子的计算方式或算法的实现，会让排序方法失效，这也是为什么目前程序使用 playwright。
