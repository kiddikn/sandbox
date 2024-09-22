package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"sort"

	"github.com/cheggaaa/pb/v3"
	"github.com/slack-go/slack"
)

const (
	RankinNumber            = 30
	ConversationTargetLimit = 200
	fileName                = "emoji_rankings.csv"
)

func main() {
	slackToken := os.Getenv("SLACK_API_TOKEN")
	if slackToken == "" {
		log.Fatal("SLACK_API_TOKEN must be set")
	}

	api := slack.New(slackToken)

	authTest, err := api.AuthTest()
	if err != nil {
		log.Fatalf("Error during AuthTest: %v", err)
	}
	fmt.Printf("Authenticated as user: %s\n", authTest.User)

	// 全てのチャンネルのリストを取得
	fmt.Println("start get all channels")
	var allChannels []slack.Channel
	cursor := ""

	for {
		params := &slack.GetConversationsParameters{
			Types:  []string{"public_channel"},
			Cursor: cursor,
		}
		channels, nextCursor, err := api.GetConversations(params)
		if err != nil {
			log.Fatalf("Error getting conversations: %v", err)
		}
		allChannels = append(allChannels, channels...)
		if nextCursor == "" {
			break
		}
		cursor = nextCursor
	}
	fmt.Printf("all channels number is %d\n", len(allChannels))

	bar := pb.Simple.Start(len(allChannels))
	bar.SetMaxWidth(80)

	emojiUsage := make(map[string]int)
	for _, channel := range allChannels {
		bar.Increment()
		if channel.IsArchived {
			continue
		}

		history, err := api.GetConversationHistory(&slack.GetConversationHistoryParameters{
			ChannelID: channel.ID,
			Limit:     ConversationTargetLimit,
		})
		if err != nil {
			log.Printf("error getting conversation history for channel %s: %v", channel.Name, err)
			continue
		}

		for _, message := range history.Messages {
			// TODO:messageの作成日時が2024年1月以降以外はスキップ
			for _, reaction := range message.Reactions {
				emojiUsage[reaction.Name] += reaction.Count
			}
		}

	}
	bar.Finish()

	type emojiCount struct {
		Name  string
		Count int
	}
	var emojiList []emojiCount
	for emoji, count := range emojiUsage {
		emojiList = append(emojiList, emojiCount{Name: emoji, Count: count})
	}
	sort.Slice(emojiList, func(i, j int) bool {
		return emojiList[i].Count > emojiList[j].Count
	})

	// すでにfileNameがあれば削除
	if _, err := os.Stat(fileName); err == nil {
		if err := os.Remove(fileName); err != nil {
			fmt.Println("Error removing CSV file:", err)
			return
		}
	}

	// カレントディレクトリにcsvを作成
	file, err := os.Create(fileName)
	if err != nil {
		fmt.Println("Error creating CSV file:", err)
		return
	}
	defer file.Close()

	// csvのstreamを作成
	writer := csv.NewWriter(file)
	defer writer.Flush()

	// ヘッダーを書き込み
	writer.Write([]string{"Rank", "Emoji", "Count"})

	fmt.Println("Emoji Usage Rankings:")
	for i, emoji := range emojiList {
		if i >= RankinNumber {
			break
		}
		writer.Write([]string{fmt.Sprintf("%d", i+1), fmt.Sprintf(":%s:", emoji.Name), fmt.Sprintf("%d", emoji.Count)})
	}
	fmt.Println("Emoji rankings saved to " + fileName)
}
