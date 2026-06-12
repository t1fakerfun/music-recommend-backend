package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath" 
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type YouTubeHistoryItem struct {
	Header    string `json:"header"`
	Title     string `json:"title"`
	TitleURL  string `json:"titleUrl"`
	Subtitles []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"subtitles"`
	Time string `json:"time"`
}

type CleanedMusicLog struct {
	UserID    int
	Title     string
	Artist    string
	URL       string
	Channel   string
	WatchedAt time.Time // 👈 time.Time 型
}

var db *sql.DB

func main() {
	var err error
	// parseTime=true を入れることで MySQL の DATETIME を Go の time.Time に自動マッピングします
	dsn := "root:password@tcp(mysql_db:3306)/music_db?parseTime=true"

	for i := 0; i < 10; i++ {
		db, err = sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("mysql の起動を待っています")
		time.Sleep(5 * time.Second)
	}

	if err != nil {
		log.Fatalf("mysql に接続できませんでした: %v", err)
	}
	defer db.Close()

	// コンテナ起動時にフォルダがなければ自動作成する
	os.MkdirAll("imports", os.ModePerm)
	os.MkdirAll("processed", os.ModePerm)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "楽曲レコメンドAPIへようこそ")
	})
	http.HandleFunc("/logs", handleSaveLog)
	http.HandleFunc("/recommend", handleRecommend)
	http.HandleFunc("/import", handleBulkImport)
	http.HandleFunc("/summary", handleGetSummary)

	log.Println("サーバーを起動します: http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("サーバーの起動に失敗しました: %v", err)
	}
}

func handleBulkImport(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	
	// 👈 2. フォルダ名だけを正しく指定。かつ末尾のjsonチェックはループ内で行う
	files, err := filepath.Glob("imports/*.json")
	if err != nil || len(files) == 0 {
		http.Error(w, "imports/ フォルダに処理対象のjsonファイルが見つかりません", http.StatusBadRequest)
		return
	}
	
	totalImported := 0
	
	for _, filePath := range files {
		// .json 以外のファイル（.DS_Storeなど）をスキップする防衛策
		if !strings.HasSuffix(filePath, ".json") {
			continue
		}
		fileBytes, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("ファイルの読み込みに失敗: %v", err)
			continue
		}

		var inputs []YouTubeHistoryItem
		if err := json.Unmarshal(fileBytes, &inputs); err != nil {
			log.Printf("Jsonのパースに失敗 (%s): %v", filePath, err)
			continue
		}

		numWorkers := runtime.NumCPU()
		inputChan := make(chan YouTubeHistoryItem, len(inputs))
		validChan := make(chan CleanedMusicLog, len(inputs))
		var wg sync.WaitGroup

		for i := 0; i < numWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range inputChan {
					isMusic := false
					artistName := "Unknown Artist"

					if len(item.Subtitles) > 0 {
						artistName = item.Subtitles[0].Name
					}

					if item.Header == "YouTube Music" || strings.Contains(artistName, " - Topic") || strings.Contains(artistName, "- トピック") {
						isMusic = true
					}

					if isMusic && item.Time != "" {
						cleanedTitle := strings.Replace(item.Title, " を視聴しました", "", 1)
						cleanedArtist := strings.Replace(artistName, " - Topic", "", 1)
						cleanedArtist = strings.Replace(cleanedArtist, " - トピック", "", 1)

						// 👈 3. 文字列(ISO8601)から time.Time 型へ安全にパース
						layout := "2006-01-02T15:04:05.999Z"
						parsedTime, err := time.Parse(layout, item.Time)
						if err != nil {
							continue
						}

						validChan <- CleanedMusicLog{
							UserID:    1,
							Title:     cleanedTitle,
							Artist:    cleanedArtist,
							URL:       item.TitleURL,
							Channel:   item.Header,
							WatchedAt: parsedTime, // 👈 パースした日時を代入
						}
					}
				}
			}()
		}

		for _, input := range inputs {
			inputChan <- input
		}

		close(inputChan)
		wg.Wait()
		close(validChan)

		var validInputs []CleanedMusicLog
		for v := range validChan {
			validInputs = append(validInputs, v)
		}

		if len(validInputs) > 0 {
			insertedRows, err := executeBulkInsert(validInputs)
			if err != nil {
				log.Printf("バルクインサートに失敗: %v", err)
				continue
			}
			totalImported += insertedRows
		}

		// 👈 4. 変数名を整理して、処理済みフォルダへ綺麗に移動
		fileName := filepath.Base(filePath)
		destPath := filepath.Join("processed", fileName)

// 1. 移動先のフォルダにファイルを丸ごと新しく書き出す（中身のコピー）
		err = os.WriteFile(destPath, fileBytes, 0644)
		if err != nil {
			log.Printf("ファイルのコピーに失敗: %v", err)
		} else {
			// 2. コピーが成功したら、元あった imports/ 側のファイルを削除する
			err = os.Remove(filePath)
			if err != nil {
				log.Printf("元ファイルの削除に失敗: %v", err)
			} else {
				log.Printf("【移動完了】%s -> %s", filePath, destPath)
			}
		}
	}
	
	duration := time.Since(startTime)
	w.WriteHeader(http.StatusOK)
	// 👈 5. totalImported（全ファイルの合計）を出力するように修正
	fmt.Fprintf(w, "完了! 新たに %d 件のレコードを効率的にインサートしました。処理時間: %v\n", totalImported, duration)
}

func executeBulkInsert(inputs []CleanedMusicLog) (int,error) {
	chunkSize := 1000
	totalInputs := len(inputs)
	actualInserted := 0

	for i := 0; i < totalInputs; i += chunkSize {
		end := i + chunkSize
		if end > totalInputs {
			end = totalInputs
		}

		chunk := inputs[i:end]
		query := "INSERT IGNORE INTO raw_watch_logs (user_id, title, artist, url, channel, watched_at) VALUES "

		valueStrings := make([]string, 0, len(chunk))
		valueArgs := make([]interface{}, 0, len(chunk)*6)

		for _, input := range chunk {
			valueStrings = append(valueStrings, "(?,?,?,?,?,?)")
			valueArgs = append(valueArgs, input.UserID, input.Title, input.Artist, input.URL, input.Channel, input.WatchedAt)
		}

		query += strings.Join(valueStrings, ",")

		res, err := db.Exec(query, valueArgs...)
		if err != nil {
			return 0, fmt.Errorf("バルクインサートの実行に失敗: %v", err)
		}

		rows, err := res.RowsAffected()
		if err == nil {
			actualInserted += int(rows)
		}

		log.Printf("バルクインサート: %d 件のレコードを処理しました", len(chunk))
	}
	return actualInserted, nil
}

type MusicSummary struct {
	Title      string `json:"title"`
	Artist     string `json:"artist"`
	WatchCount int    `json:"watch_count"`
}

func handleGetSummary(w http.ResponseWriter, r *http.Request) {
	query := `
		SELECT title, artist, COUNT(*) as watch_count
		FROM raw_watch_logs
		GROUP BY url, title, artist
		ORDER BY watch_count DESC
		LIMIT 10`
	rows, err := db.Query(query)
	if err != nil {
		http.Error(w, fmt.Sprintf("データベースクエリに失敗: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var summaries []MusicSummary
	for rows.Next() {
		var s MusicSummary
		if err := rows.Scan(&s.Title, &s.Artist, &s.WatchCount); err != nil {
			http.Error(w, fmt.Sprintf("データのスキャンに失敗: %v", err), http.StatusInternalServerError)
			return
		}
		summaries = append(summaries, s)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func handleSaveLog(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	historyIDStr := r.URL.Query().Get("history_id")

	userID, err1 := strconv.Atoi(userIDStr)
	historyID, err2 := strconv.Atoi(historyIDStr)

	if err1 != nil || err2 != nil {
		http.Error(w, "user_id と history_id は有効な数値で指定してください", http.StatusBadRequest)
		return
	}

	var exists bool
	checkQuery := "SELECT EXISTS(SELECT 1 FROM watch_history WHERE id = ?)"
	err := db.QueryRow(checkQuery, historyID).Scan(&exists)
	if err != nil {
		http.Error(w, "データベースエラーが発生しました", http.StatusInternalServerError)
		return
	}

	if !exists {
		http.Error(w, fmt.Sprintf("エラー：指定された楽曲ID（%d）はマスターデータに存在しません", historyID), http.StatusBadRequest)
		return
	}

	insertQuery := "INSERT INTO listening_logs (user_id, history_id) VALUES (?, ?)"
	_, err = db.Exec(insertQuery, userID, historyID)
	if err != nil {
		http.Error(w, fmt.Sprintf("ログの保存に失敗しました: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "[成功] ユーザーID: %d が 楽曲ID: %d を視聴したファクトを強固に記録しました！", userID, historyID)
}

func handleRecommend(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id は必須です", http.StatusBadRequest)
		return
	}

	cmd := exec.Command("python3", "recommend.py", userID)
	output, err := cmd.Output()
	if err != nil {
		http.Error(w, fmt.Sprintf("AIモデルの呼び出しに失敗しました: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(output)
}