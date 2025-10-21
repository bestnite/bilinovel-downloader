package test

import (
	"fmt"
	"log"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// unscrambleParagraphs 函数的核心功能是接收一个乱序的段落列表，
// 并根据 chapterID 将它们重新排序为正确的阅读顺序。
// 算法来源 https://www.bilinovel.com/themes/zhmb/js/chapterlog.js?v1006c1
// 反混淆工具 https://obf-io.deobfuscate.io http://jsnice.org
// 这个方案是可行的，但如果 bilinovel 频繁更改初始化种子的计算方式或算法的实现，会让排序方法失效，可能 playwright 还是最优解。
func unscrambleParagraphs(scrambledParagraphs []*goquery.Selection, chapterID int) []*goquery.Selection {
	j := len(scrambledParagraphs)
	// 根据JS逻辑，如果段落数小于等于20，则不进行排序
	if j <= 20 {
		return scrambledParagraphs
	}

	// 1. 精确复刻JS中的伪随机数生成器和洗牌算法，以得到正确的索引映射关系。
	// 初始化种子
	ms := int64(chapterID*127 + 235)

	// value 数组存放的是需要被打乱的、从20开始的段落的相对索引（0, 1, 2...）
	value := make([]int, j-20)
	for i := range value {
		value[i] = i
	}

	// 执行与JS完全相同的 Fisher-Yates-like 洗牌算法
	for i := len(value) - 1; i > 0; i-- {
		ms = (ms*9302 + 49397) % 233280
		prop := int(float64(ms) / 233280.0 * float64(i+1))
		// 交换元素
		value[i], value[prop] = value[prop], value[i]
	}

	// 2. 构建最终的索引映射表 (aProperties)。
	// 这个表告诉我们，乱序列表中的每一项，应该被放到正确顺序列表的哪个位置。
	aProperties := make([]int, j)
	// 前20个段落顺序不变
	for i := range 20 {
		aProperties[i] = i
	}
	// 后续的段落使用洗牌后的索引，并加上20的偏移量
	for i := range value {
		aProperties[i+20] = value[i] + 20
	}

	// 3. 根据索引映射关系，从乱序列表中恢复出正确顺序。
	// JS逻辑: elements[aProperties[i]] = out[i].node
	// 翻译过来就是：乱序列表中的第 `i` 项 (scrambledParagraphs[i])，
	// 它在最终排好序的列表中的正确位置应该是 `aProperties[i]`。
	correctlyOrdered := make([]*goquery.Selection, j)
	for i := range j {
		correctPosition := aProperties[i]
		correctlyOrdered[correctPosition] = scrambledParagraphs[i]
	}

	return correctlyOrdered
}

func TestResortDom(t *testing.T) {
	// --- 步骤 1: 准备原始HTML ---
	// 请将您用 http 请求获取到的、未经处理的完整HTML源码粘贴到这里。
	// 这里使用的是您之前提供的原始HTML作为示例。
	unprocessedHtmlContent := `
<!DOCTYPE html>
<html lang="zh-Hans">
<head>
<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />
<title>女主角？ 圣女？ 不，我是全业女仆（自豪）！ 第１章 第１话 目标成为女仆的少女_哔哩轻小说</title>
<meta name="keywords" content="女主角？ 圣女？ 不，我是全业女仆（自豪）！,第１话 目标成为女仆的少女,哔哩轻小说" />
<meta name="description" content="哔哩轻小说提供 あてきち 所创作的 女主角？ 圣女？ 不，我是全业女仆（自豪）！ 第１章 第１话 目标成为女仆的少女 在线阅读与TXT,epub下载" />
<meta name="viewport" content="initial-scale=1.0,minimum-scale=1.0,user-scalable=yes,width=device-width" />
<meta name="theme-color" content="#232323" media="(prefers-color-scheme: dark)" />
<meta name="applicable-device" content="mobile" />
<link rel="stylesheet" href="https://www.bilinovel.com/themes/zhmb/css/read.css?v0409c2">
<link rel="stylesheet" href="https://www.bilinovel.com/themes/zhmb/css/chapter.css?v1126a9">
<link rel="dns-preconnect" href="https://www.bilinovel.com">
<link rel="alternate" hreflang="zh-Hant" href="https://tw.linovelib.com/novel/4126/236197.html" />
<script src="https://www.bilinovel.com/themes/zhmb/js/jquery-3.3.1.js"></script>
<script type="text/javascript" src="/scripts/darkmode.js"></script>
<script async src="https://www.bilinovel.com/themes/zhmb/js/lazysizes.min.js"></script>
<script src="https://www.bilinovel.com/scripts/common.js?v0922a3"></script>
<script src="https://www.bilinovel.com/scripts/zation.js?v1004a4"></script>
<style>.center-note{text-align: center; margin: 0; height: 50vh; display: flex ; justify-content: center; align-items: center;}.sum1{display:none}.footlink a{box-shadow: 0 0 1px rgba(150,150,150,.6);}.footlink a:nth-child(1){display: inline-block;margin-bottom: 10px;width: 90%;}.footlink a:nth-child(2){padding: 5px 10px;float: left;width: 35%;margin-left: 5%;}.footlink a:nth-child(3){padding: 5px 10px;float: right;width: 35%;margin-right: 5%;}.footlink a:nth-child(4){display: inline-block;margin-top: 10px;width: 90%;}#acontent{text-align: unset;}</style>
<script type="text/javascript">var ual = navigator.language.toLowerCase();var isWindows = navigator.platform.toLowerCase().includes("win");if(ual == 'zh-tw' || ual == 'zh-hk'){window.location.replace("https://tw.linovelib.com/novel/4126/236197.html");}if (ual === 'zh-cn' && isWindows) { window.location.replace("https://www.linovelib.com/novel/4126/236197.html");}</script>
</head>
<body id="aread">
<script type="text/javascript">var ReadParams={url_previous:'/novel/4126/236196.html',url_next:'/novel/4126/236197_2.html',url_index:'/novel/4126/catalog',url_articleinfo:'/novel/4126/vol_236194.html',url_image:'https://www.bilinovel.com/files/article/image/4/4126/4126s.jpg',url_home:'https://www.bilinovel.com/',articleid:'4126',articlename:'女主角？ 圣女？ 不，我是全业女仆（自豪）！',subid:'/4',author:'あてきち',chapterid:'236197',page:'1',chaptername:'第１章 第１话 目标成为女仆的少女',chapterisvip:'0',userid:'0',readtime:'1761057661'}</script>
<div class="main">
	<div id="abox" class="abox">
	<div id="apage" class="apage">
	    <div class="atitle"><h1 id="atitle">第１话 目标成为女仆的少女</h1><h3>第１章</h3></div>
		<div id="acontent" class="contente"><div class="cgo"><!--<script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-8799828951681010"
	 crossorigin="anonymous"></script>
<ins class="adsbygoogle"
	 style="display:block"
	 data-ad-client="ca-pub-8799828951681010"
	 data-ad-slot="2277430192"
	 data-ad-format="auto"
	 data-full-width-responsive="true"></ins>
<script>
	 (adsbygoogle = window.adsbygoogle || []).push({});
</script>--></div><p>「欢迎回来，老爷。」</p>
<br>
<p>一位少女恭敬地弯腰向走进木质大门的绅士致意。</p>
<p>少女穿着一件做工精致的黑色连衣裙，上面系着花边以及刺绣、并不华丽的纯白围裙，梳成编辫的黑发上系着可爱的蕾丝头带。</p>
<br>
<p>无论从哪个角度看，都是迎接主人归来的女仆样子。</p>
<br>
<p>「啊，我回来了」</p>
<br>
<p>绅士把帽子和大衣交给恭敬地弯腰的女仆，用温柔的语气回答。</p>
<br>
<p>「我马上为您准备茶水。请问您想要哪一款？」</p>
<p>「那么，我想要一杯伯爵红茶。」</p>
<p>「要加牛奶之类的吗？」</p>
<p>「不，不用了。」</p>
<p>「遵命。茶点要什么呢？」</p>
<p>「嗯，就交给你吧。拜托了？」</p>
<br>
<p>对着绅士的话语，身为女仆的少女露出了轻柔的微笑。她可能只有十五、六岁吧。脸上还带着稚气，但未来值得期待，可爱又温柔的容貌。</p>
<br>
<p>「请交给我，我会准备合您口味的茶点。」</p>
<p>「啊，拜托了。」</p>
<br>
<p>女仆少女将帽子和大衣挂在衣架上，然后引导绅士到餐桌。</p>
<br>
<p>＊＊＊＊＊＊＊＊＊＊</p>
<br>
<p>「那么，我要出门了。」</p>
<p>「好的，老爷」</p>
<p>「下次回来时，如果能再让妳接待就好了……」</p>
<br>
<p>「下次她想要带朋友在露台喝茶，也希望你能照顾他们。」</p>
<br>
<p>轻轻敲门后，听到「请进」的回答，少女走进了房间行礼。</p>
<p>一个少女嘟囔着。那是一位身穿简素蓝色连衣裙的少女。闪闪发光的银色头发留到了胸口。有着神秘的琉璃色瞳孔的美丽可爱少女站在母亲身旁。</p>
<p>送走绅士后，女仆少女前往总管的房间。</p>
<br>
<p>「欸，对我不需要用这种说话方式吧？……律子酱。」</p>
<p>薪水丰厚的兼职让她顺利存下了留学费用，留学之日即将到来。</p>
<p>「拜托了！」</p>
<br>
<p>女仆少女律子满脸笑容地回答。</p>
<br>
<p>「话说回来，律子酱。上次来的坂上夫人很喜欢你呢。上次寄来的邮件里相当称赞。她说下次还打算指名。」</p>
<br>
<p>「失礼了，Miss 阿曼达。关于刚才离开宅邸的老爷报告……」</p>
<br>
<p>被叫做律子的女仆少女张开眼，刚才还散发着女仆气息的模样一下子变回稚气十足的少女，她嘟起嘴说道。</p>
<br>
<p>「这样很好啊！」</p>
<p>女仆少女律子满脸笑容地回答。</p>
<p>对担忧这一点的父母来说，当时的律子的情况无疑让人开心。</p>
<br>
<p>因此，父母并未反对女儿出人意表的宣言。</p>
<br>
<p>标题叫『深窗的公主的悲恋』。</p>
<p>优雅的动作，没有任何不自然的温柔笑容。仿佛是女仆典范一般的少女。看着她的身影，总管阿曼达皱了皱眉。不，这是因为……</p>
<p>「怎么了？瑟蕾丝蒂？」</p>
<br>
<p>「啊，拜托了。那么……」</p>
<p>「一路顺风，老爷。」</p>
<p>「遵命。我会将您的意愿转达给<ruby>女仆总管<rp>(</rp><rt>家政妇</rt><rp>)</rp></ruby>。」</p>
<p>（公主身后的女仆们是多么的优秀啊！）</p>
<br>
<p>「你真的很喜欢做这种工作呢。这样一来就得早晨开始准备了。下次我会去问问她们的希望。」</p>
<br>
<p>这部电影以旧时英国贵族的故事为题材。描述了一位在呵护下长大的贵族千金，偶然认识一位平民青年，并陷入爱河的故事。最后，因为身份差异，两人自尽，悲剧结局。</p>
<br>
<p>父母看着律子的身影，感到非常开心。</p>
<p>女仆们使出各种手段帮助她与男子相会。</p>
<p>在女仆的影响下，律子对各种事物产生了兴趣，玩耍、笑声、学习，成长为一个非常优秀的女儿。自从遇见女仆以来，好奇心无止境，虽然年龄和性格相比有些幼稚，但对父母来说，女仆这个存在也是让人有好感的。</p>
<br>
<p>她的名字是瑞波律子，二十岁，现在是大学二年级的学生。</p>
<br>
<p>「我讨厌那个名字啊。明明是日本人，却叫阿曼达……」 <span style="color: rgb(61，142，185);">（＊亚万田日语念成阿曼达）</span></p>
<br>
<p>当然，因为主角是英国贵族千金，所以电影里并没有描绘女仆们努力的场景。但正因为如此，律子对在幕后默默支持的女仆们十分感动。</p>
<p>「……本来应该是这样的啊。」</p>
<p>来这家女仆咖啡厅的客人并不仅仅是男性。这家店的男女客人比例几乎是一比一。</p>
<br>
<p>会员制高级女仆咖啡厅『<ruby>贵族的日常<rp>(</rp><rt>Noble's One Day</rt><rp>)</rp></ruby>』。</p>
<br>
<p>生活了六年，律子慢慢的成长，但她却不对事物报持热情。喜欢的玩具和书籍都没有，看电视也不会表现出太多兴趣。</p>
<br>
<p>「拜托了！」</p>
<br>
<p>那是瑞波律子还不懂爱情的六岁春天的事……先不管给一个六岁小孩看悲恋电影的问题。</p>
<p>是被称为女仆总管的女性，亚万田凪沙创建的店。</p>
<br>
<p>「我在大学毕业后，想在英国成为真正的女仆！」</p>
<br>
<p>「好的，请放心交给我！」</p>
<br>
<p>「欸，真的吗!?  就是上周来过的那位温柔的女士吗？」</p>
<p>男士需穿着西装，女士需穿着礼服，这是服装规定。特别为女性客人提供服装租赁服务，因此女性客人可以享受穿着平时难得一穿的贵族少女或贵妇风格的洋装，扮演女主人的角色。</p>
<br>
<p>虽然二十岁了，律子的脸庞略显年幼，她是这家店最受欢迎的女仆。</p>
<br>
<p>看过这部电影的观众都为两人的悲恋流泪，感动不已。</p>
<br>
<p>从那时起，律子就迷上了女仆。她向父母说明了女仆是多么伟大的存在，并激动地宣布有一天她也会成为女仆。</p>
<p>完全预约制，到店时会有指名的女仆迎接。此时店员会完全扮演女仆角色，客人不是客人身份，而是扮演女仆的主人，享受其中。</p>
<br>
<p>一切都是顺风顺水。距离成为女仆只剩下最后一步！</p>
<br>
<p>美丽的行礼后，少女向绅士回以温柔的微笑。绅士推开门离开了。</p>
<br>
<p>律子的梦想是成为女仆。原因非常简单，那是因为她小时候看过的一部电影。</p>
<br>
<p>在父母的支持下，律子在大学学习外语、历史、文学、礼仪等，以成为女仆为目标，在本格派女仆咖啡厅进行女仆训练的日常。</p>
<br>
<p>「那么，我也可以帮忙准备衣服和化妆吗？」</p>
<p>「讨厌！再让我扮一下女仆也没关系嘛，亚万田小姐！」</p>
<br>
<p>绅士略显羞涩地说着，女仆的少女露出了微笑回答。</p>
<p>然而，律子却对另一方面感动不已。</p>
<br>
<br>
<br>
<br>
<br>
<p>支付是预付制，店内不谈金钱。没有菜单，女仆会自然接受点单。客人只需要享受那片刻的主人时光即可。</p>
<p>女主角的贵族千金拥有很温柔的人格，所以她的女仆们也非常喜爱她。</p>
<br>
<p>为了筹集到英国留学的资金，进入大学的律子开始寻找兼职工作。她认为对未来有帮助的工作是最好的，于是找到了这家女仆咖啡厅。</p><div class="cgo"><script async src="https://pagead2.googlesyndication.com/pagead/js/adsbygoogle.js?client=ca-pub-8799828951681010"
     crossorigin="anonymous"></script>
<ins class="adsbygoogle"
     style="display:block"
     data-ad-client="ca-pub-8799828951681010"
     data-ad-slot="9085546976"
     data-ad-format="auto"
     data-full-width-responsive="true"></ins>
<script>
     (adsbygoogle = window.adsbygoogle || []).push({});
</script></div>
		</div>
	</div>
	</div>
	<div id="toptext" class="toptext" style="display:none;"></div>
	<div id="bottomtext" class="bottomtext" style="display:none;"></div>
	<div id="operatetip" class="operatetip" style="display:none;" onclick="this.style.display='none';">
		<div class="tipl"><p>翻上页</p></div>
		<div class="tipc"><p>呼出功能<br><br><small>漫画＆插图<br>建议使用上下翻页</small><br><br><small>【翻页模式】章评·默认隐藏</small></p></div>
		<div class="tipr"><p>翻下页</p></div>
	</div>
</div>
<div id="footlink" class="footlink"><a onclick="window.location.href = ReadParams.url_previous;">序章 路多帕克家的大小姐以及万能女仆</a><a onclick="window.location.href = ReadParams.url_index;">目录</a><a onclick="window.location.href = ReadParams.url_articleinfo;">书页</a><a onclick="window.location.href = ReadParams.url_next;">下一頁</a></div>

<script>$(document).ready(function(){var prevpage="/novel/4126/236196.html";var nextpage="/novel/4126/236197_2.html";var bookpage="/novel/4126.html";$("body").keydown(function(event){var isInput=event.target.tagName==='INPUT'||event.target.tagName==='TEXTAREA';if(!isInput){if(event.keyCode==37){location=prevpage}else if(event.keyCode==39){location=nextpage}}})});</script>
<script type="text/javascript" src="https://www.bilinovel.com/themes/zhmb/js/readtools.js?42sfaj-8"></script>
<script type="text/javascript" src="https://www.bilinovel.com/scripts/json2.js"></script>
<script type="text/javascript" src="https://www.bilinovel.com/themes/zhmb/js/chapterlog.js?v1006c1"></script>

<script async src="https://www.googletagmanager.com/gtag/js?id=G-1K4JZ603WH"></script>
<script>
  window.dataLayer = window.dataLayer || [];
  function gtag(){dataLayer.push(arguments);}
  gtag('js', new Date());
  gtag('config', 'G-1K4JZ603WH');
</script>
<script>
var _hmt = _hmt || [];
(function() {
  var hm = document.createElement("script");
  hm.src = "https://hm.baidu.com/hm.js?6f9595b2c4b57f95a93aa5f575a77fb0";
  var s = document.getElementsByTagName("script")[0]; 
  s.parentNode.insertBefore(hm, s);
})();
</script>
<!--<script>
  if ('serviceWorker' in navigator) {
      navigator.serviceWorker.getRegistrations().then(function(registrations) {
        for (let registration of registrations) {
          registration.unregister();
        }
      });
  }
</script>-->
<script defer src="https://static.cloudflareinsights.com/beacon.min.js/vcd15cbe7772f49c399c6a5babf22c1241717689176015" integrity="sha512-ZpsOmlRQV6y907TI0dKBHq9Md29nnaEIPlkf84rnaERnq6zvWvPUqr2ft8M1aS28oN72PdrCzSjY4U6VaAw1EQ==" data-cf-beacon='{"version":"2024.11.0","token":"192783771d59492782cd05bd12eb61b9","r":1,"server_timing":{"name":{"cfCacheStatus":true,"cfEdge":true,"cfExtPri":true,"cfL4":true,"cfOrigin":true,"cfSpeedBrain":true},"location_startswith":null}}' crossorigin="anonymous"></script>
</body>
</html>`

	// --- 步骤 2: 解析HTML并提取关键信息 ---
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(unprocessedHtmlContent))
	if err != nil {
		log.Fatalf("解析HTML失败: %v", err)
	}

	chapterID := 236197

	// --- 步骤 3: 收集所有需要重排的段落 ---
	var scrambledParagraphs []*goquery.Selection
	doc.Find("#acontent p").Each(func(i int, s *goquery.Selection) {
		// 确保只添加非空段落，与JS逻辑保持一致
		if len(strings.TrimSpace(s.Text())) > 0 {
			scrambledParagraphs = append(scrambledParagraphs, s)
		}
	})
	fmt.Printf("从原始HTML中找到 %d 个乱序段落，准备重排。\n\n", len(scrambledParagraphs))

	// --- 步骤 4: 执行重排算法 ---
	correctlyOrderedParagraphs := unscrambleParagraphs(scrambledParagraphs, chapterID)

	// --- 步骤 5: 输出最终结果 ---
	fmt.Println("--- 已恢复正确顺序的最终内容 ---")
	for i, p := range correctlyOrderedParagraphs {
		fmt.Printf("%d: %s\n", i+1, p.Text())
	}
}
