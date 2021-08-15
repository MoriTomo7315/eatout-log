package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Data struct {
	Events []Event `json:"data"`
}

type Event struct {
	Id         string     `json:"id"`
	Type       string     `json:"type"`
	Attributes Attributes `json:"attributes"`
}

type Attributes struct {
	Title         string    `json:"title"`
	AllDay        bool      `json:"all_day"`
	StartAt       time.Time `json:"start_at"`
	StartTimezone string    `json:"start_timezone"`
	EndAt         time.Time `json:"end_at"`
	EndTimezone   string    `json:"end_timezone"`
	Location      string    `json:"location"`
	LocationLat   string    `json:"location_lat"`
	LocationLon   string    `json:"location_lon"`
	Url           string    `json:"url"`
	UpdatedAt     string    `json:"updated_at"`
	CreatedAt     string    `json:"created_at"`
	Category      string    `json:"category"`
	Description   string    `json:"description"`
	Recurrence    string    `json:"recurrence"`
	RecurringUuid string    `json:"recurring_uuid"`
}

type Eatout struct {
	Name   string    `json:"name"`
	Lat    string    `json:"lat"`
	Lon    string    `json:"lon"`
	WentAt time.Time `json:"went_at`
}

func main() {
	os.Exit(run(os.Args))
}

func sqlConnect() (database *gorm.DB, err error) {
	USER := "moritomo"
	PASS := "moritomo_mountain"
	PROTOCOL := "tcp(localhost:3306)"
	DBNAME := "eatout_log"

	DSN := USER + ":" + PASS + "@" + PROTOCOL + "/" + DBNAME + "?charset=utf8&parseTime=true&loc=Asia%2FTokyo"
	return gorm.Open(mysql.Open(DSN), &gorm.Config{})
}

func getEatout(name string) Eatout {
	var eatout Eatout
	db, err := sqlConnect()
	if err != nil {
		panic(err.Error())
	} else {
		fmt.Println("DB接続成功")
		db.Raw("SELECT * FROM eatouts WHERE name = ? limit 1", name).Scan(&eatout)
		return eatout
	}
}

func insertEatout(eatout Eatout) {
	db, err := sqlConnect()
	if err != nil {
		panic(err.Error())
	} else {
		fmt.Println("DB接続成功")
		db.Create(&eatout)
	}
}

func run(args []string) int {
	// .envファイルの読み込み
	err := godotenv.Load(fmt.Sprintf("../%s.env", os.Getenv("GO_ENV")))
	if err != nil {
		// .env読めなかった場合の処理
		fmt.Printf(".envファイル読み込み失敗")
		return 0
	}

	// timetree apiのtokenを取得
	timetree_api_token := os.Getenv("TIMETREE_API_TOKEN")
	// timetree のカレンダーID取得
	timetree_calender_id := os.Getenv("TIMETREE_CALENDER_ID")
	// timetree apiのendpoint
	timetree_api_endpoint := "https://timetreeapis.com/calendars/" + timetree_calender_id + "/upcoming_events?days=7"
	// httpのクライアントを作成する
	client := &http.Client{}
	// タイムアウトの設定をしたほうがいいみたい
	client.Timeout = time.Second * 15

	// リクエストを作成
	req, err := http.NewRequest("GET", timetree_api_endpoint, nil)
	if err != nil {
		return 1
	}

	// リクエストヘッダーを指定
	req.Header.Add("Accept", "application/vnd.timetree.v1+json")
	req.Header.Add("Authorization", "Bearer "+timetree_api_token)

	// リクエストを実行
	resp, err := client.Do(req)
	if err != nil {
		return 2
	}
	defer resp.Body.Close()

	// レスポンスの読み込み
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 3
	}

	var data Data
	// レスポンスボディをJSONパース
	json.Unmarshal([]byte(body), &data)

	// Eventデータ抜き出し
	events := data.Events

	// レスポンスから外食予定を抜き出し
	// 外食の予定はtitleに"[EO]"とつけているのでそれで判断
	var eo_indexes []int
	for i := range events {
		if strings.Contains(events[i].Attributes.Title, "[EO]") {
			eo_indexes = append(eo_indexes, i)
		}
	}

	// 外食予定のlocationと、座標、日付だけ抜き取る
	var eatouts []Eatout
	for i := range eo_indexes {
		eatouts = append(eatouts, Eatout{
			events[eo_indexes[i]].Attributes.Location,
			events[eo_indexes[i]].Attributes.LocationLat,
			events[eo_indexes[i]].Attributes.LocationLon,
			events[eo_indexes[i]].Attributes.StartAt})
	}

	fmt.Println(eatouts)

	// すでに予定はDBに取り込み済みか確認
	var selectEatout Eatout
	var do_insert_indexes []int
	for i := range eatouts {
		selectEatout = getEatout(eatouts[i].Name)
		// レコードが選択されない場合、まだ外食先はDBに登録されていないので登録。
		// レコードが選択されなかった場合のEatout = {   0001-01-01 00:00:00 +0000 UTC}
		if selectEatout.Name == "" {
			fmt.Println("%sを取り込み", eatouts[i].Name)
			do_insert_indexes = append(do_insert_indexes, i)
		}
	}

	// 未取り込みの外食先がある場合は取り込む
	if len(do_insert_indexes) > 0 {
		fmt.Println("%d件取り込み開始", len(do_insert_indexes))
		for i := range do_insert_indexes {
			insertEatout(eatouts[do_insert_indexes[i]])
			fmt.Println(eatouts[do_insert_indexes[i]])
		}
		fmt.Println("%d件取り込み終了", len(do_insert_indexes))
	} else {
		fmt.Println("取り込み対象が見つかりませんでした。")
	}

	return 0
}
