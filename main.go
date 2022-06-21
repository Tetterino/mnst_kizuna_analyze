package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var year string
var month string
var day string

var member = [][]string{
	{"", ""}, // {検索文字列,検索除外文字列}
}

var memberEvalMap = map[string]string{
	"": "", // "name", ""
}

type analyzeResult struct {
	SpontaneousCnt int    // メンションを10人以上同時にしたメッセージ数
	ReplyCnt       int    // メンション3人以下のメッセージ数
	MessageCnt     int    // メッセージ総数
	FromMention    int    // メンションされた回数
	FisrtMsgTime   string // 1番最初のメッセージ時間
	LastMsgTime    string // 1番最後のメッセージ時間
}

func main() {
	// 日付等の初期化処理
	initDate()

	// 1次ファイル生成
	makePreFile()

	// 対象日が月初ならtsvを生成
	if day == "01" {
		makeTsv()
	}

	// 各個人ごとに解析
	/* ISSUE : 1次ファイル1行ずつ解析する方が効率いいけど...
	   考慮しなきゃいけないことが多すぎて一旦放置... */
	analyze()

	eval()
}

func initDate() {
	// 日付を標準入力で受け取る(省略した場合は実行日の前日)
	dateflg := flag.String("date", time.Now().AddDate(0, 0, -1).Format("20060102"), "日付")
	flag.Parse()

	if len(*dateflg) != 8 {
		fmt.Println("引数が不正です。")
		os.Exit(1)
	} else if _, err := strconv.Atoi(*dateflg); err != nil {
		fmt.Println("引数が日付になっていません。")
		os.Exit(1)
	}

	fmt.Printf("%s のトーク履歴を解析します。\n", *dateflg)

	// 日付パース
	date := *dateflg // メモリコピー
	year = date[0:4]
	month = date[4:6]
	day = date[6:8]
}

func check_regexp(reg, str string) bool {
	return regexp.MustCompile(reg).Match([]byte(str))
}

func makePreFile() {
	fmt.Println("1次ファイルの作成を開始します")
	defer fmt.Printf("1次ファイルの作成完了しました。\nPATH: ./output/%s/%s/%s.txt\n", year, month, day)
	// ファイル読み込み
	inputFile, err := os.Open(fmt.Sprintf("./input/%s/%s/%s.txt", year, month, day))
	if err != nil {
		fmt.Println("ファイル読み込みに失敗しました。")
		os.Exit(1)
	}
	scanner := bufio.NewScanner(inputFile)

	// ディレクトリ作成・1次ファイル作成
	if err := os.MkdirAll(fmt.Sprintf("./output/%s/%s", year, month), os.ModePerm); err != nil {
		fmt.Println("ディレクトリの作成に失敗しました。")
		os.Exit(1)
	}
	outputFile, err := os.Create(fmt.Sprintf("./output/%s/%s/%s.txt", year, month, day))
	if err != nil {
		fmt.Println("1次ファイル作成に失敗しました。")
		os.Exit(1)
	}
	displayFlg := false
	for scanner.Scan() {
		line := scanner.Text()
		if check_regexp(`(?m)^[0-9]{4}\.[0-9]{2}\.[0-9]{2}`, line) {
			if strings.Contains(line, fmt.Sprintf("%s.%s.%s", year, month, day)) {
				displayFlg = true
			} else {
				displayFlg = false
			}
		}

		if !displayFlg {
			continue
		}

		if check_regexp(`(?m)^[0-9]{2}:[0-9]{2}`, line) {
			outputFile.WriteString(fmt.Sprintf("\n%s", line))
		} else {
			outputFile.WriteString(line)
		}
	}
	inputFile.Close()
	outputFile.Close()
}

func analyzeByPerson(name, exclusionStr string) (string, string) {
	reslut := analyzeResult{}

	inputFile, err := os.Open(fmt.Sprintf("./output/%s/%s/%s.txt", year, month, day))
	if err != nil {
		fmt.Println("ファイル読み込みに失敗しました。")
		os.Exit(1)
	}
	defer inputFile.Close()
	scanner := bufio.NewScanner(inputFile)
	for scanner.Scan() {
		line := scanner.Text()
		var messages [][]string
		if exclusionStr == "" {
			messages = grep_regexp(fmt.Sprintf(`[0-9]{2}:[0-9]{2} %s(.*)`, name), line)
		} else {
			messages = grep_regexp(fmt.Sprintf(`[0-9]{2}:[0-9]{2} %s[^%s](.*)`, name, exclusionStr), line)
		}

		if len(messages) != 0 {
			// messages[0][0] 日付部分含む (マッチ全体)
			// messages[0][1] 日付部分含まない (グループ1)
			if len(reslut.FisrtMsgTime) == 0 {
				reslut.FisrtMsgTime = messages[0][0][0:5] // 初回時間
			}
			reslut.LastMsgTime = messages[0][0][0:5] // 最終時間

			if strings.Count(messages[0][1], " @") >= 10 {
				reslut.SpontaneousCnt++
			} else if strings.Count(messages[0][1], "@") <= 3 {
				reslut.ReplyCnt++
			}

			reslut.MessageCnt++
		} else if check_regexp(name, line) {
			reslut.FromMention++
		}
	}

	return fmt.Sprintf(strings.Join([]string{
		"解析結果 : %s\n",
		"募集メッセージ : %d (メンションが10人以上)\n",
		"返信メッセージ : %d (メンションが3人以下)\n",
		"メッセージ総数 : %d\n",
		"メンションされた回数 : %d\n",
		"初回メッセージ時間 : %s\n",
		"最終メッセージ時間 : %s\n",
	}, ""),
		name,
		reslut.SpontaneousCnt,
		reslut.ReplyCnt,
		reslut.MessageCnt,
		reslut.FromMention,
		reslut.FisrtMsgTime,
		reslut.LastMsgTime), reslut.eval()
}

func grep_regexp(reg, str string) [][]string {
	return regexp.MustCompile(reg).FindAllStringSubmatch(str, -1)
}

func analyze() {
	// ディレクトリ作成
	if err := os.MkdirAll(fmt.Sprintf("./reslut/%s/%s", year, month), os.ModePerm); err != nil {
		fmt.Println("ディレクトリの作成に失敗しました。")
		os.Exit(1)
	}
	resultFile, err := os.Create(fmt.Sprintf("./reslut/%s/%s/%s.txt", year, month, day))
	if err != nil {
		fmt.Println("結果ファイル作成に失敗しました。")
		os.Exit(1)
	}
	defer resultFile.Close()
	// ISSUE: 本当は *os.File をそのまま渡したいけど、関数のスコープが大きくなるから面倒
	fmt.Println("解析開始します")
	defer fmt.Println("解析完了しました")
	resultFile.WriteString(fmt.Sprintf("%s/%s/%s\n", year, month, day))
	for _, person := range member {
		sentence, eval := analyzeByPerson(person[0], person[1])
		memberEvalMap[person[0]] = eval
		resultFile.WriteString(sentence)
		resultFile.WriteString(fmt.Sprintln(""))
	}
}

func (a *analyzeResult) eval() string {
	// メッセージ0 メンションされた数0 → 休暇
	if a.MessageCnt == 0 && a.FromMention == 0 {
		return "休暇"
	}

	// メッセージ0
	if a.MessageCnt == 0 {
		// メンションされた数5以下 → 誤メンションの可能性がある : 休暇?
		if a.FromMention <= 5 {
			return "休暇？"
			// メンションされた数6 ~ 9 → 途中で休暇にした可能性/単純に参加頻度が低くてメンションされづらい : サボり可能性・高
		} else if a.FromMention > 5 && a.FromMention < 10 {
			return "サボり可能性・高"
			// メンションされた数10以上 → サボり
		} else {
			return "サボり"
		}
	}

	// メッセージ10以下
	if a.MessageCnt <= 10 {
		// メンションされた数5以下 → 誤メンションの可能性がある : 休暇?
		if a.FromMention <= 5 {
			return "休暇？"
			// メンションされた数6 ~ 9 → わからん。
		} else if a.FromMention > 5 && a.FromMention < 10 {
			return "判断不可"
			// メンションされた数10以上 → サボり
		} else {
			if a.ReplyCnt < 5 {
				return "サボり"
			}
			return "判断不可"
		}
	}

	// 自発1回以上 返答 30回以上
	if a.SpontaneousCnt != 0 && a.ReplyCnt >= 30 {
		return "秀"
	}

	// 自発1回以上 返答 30回未満
	if a.SpontaneousCnt != 0 && a.ReplyCnt < 30 {
		return "優"
	}

	// 自発なし 返答 30回以上
	if a.SpontaneousCnt == 0 && a.ReplyCnt >= 30 {
		return "良"
	}

	// 良くも悪くもない。
	return "可"
}

func makeTsv() {
	// ディレクトリ作成
	if err := os.MkdirAll(fmt.Sprintf("./reslut/%s/%s", year, month), os.ModePerm); err != nil {
		fmt.Println("ディレクトリの作成に失敗しました。")
		os.Exit(1)
	}
	tsvFile, err := os.Create(fmt.Sprintf("./reslut/%s/%s/sabotage.tsv", year, month))
	if err != nil {
		fmt.Println("結果ファイル作成に失敗しました。")
		os.Exit(1)
	}
	defer tsvFile.Close()

	tsvHeader := "date"
	for _, person := range member {
		tsvHeader = fmt.Sprintf("%s\t%s", tsvHeader, person)
	}
	tsvHeader = fmt.Sprintln(tsvHeader)

	tsvFile.WriteString(tsvHeader)
}

func eval() {
	f, err := os.OpenFile(fmt.Sprintf("./reslut/%s/%s/sabotage.tsv", year, month), os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("ファイル読み込みに失敗しました。")
		os.Exit(1)
	}
	defer f.Close()

	tsvLine := fmt.Sprintf("%s/%s/%s", year, month, day)
	for _, person := range member {
		tsvLine = fmt.Sprintf("%s\t%s", tsvLine, memberEvalMap[person[0]])
	}
	tsvLine = fmt.Sprintln(tsvLine)

	f.WriteString(tsvLine)
}
