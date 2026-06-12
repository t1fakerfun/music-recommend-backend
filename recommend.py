import sys
import json
import mysqlconnector
import os
import google.generativeai as genai

api_key = os.getenv("GEMINI_API_KEY")
def get_recommendations(user_id):
    try:
        connection = mysql.connector.connect(
            host="mysql_db",
            user="root",
            password = "password"
            database = "music_db"
        )
        cursor = connection.cursor(dictionary = True)

        artist_query = """
            SELECT artist, COUNT(*) as play_count
            FROM raw_watch_logs
            WHERE user_id = %s
            GROUP BY artist
            GROUP BY play_count DESC
            LIMIT 3
            """
        cursor.execute(artist_query,(user_id,))
        top_artists = cursor.fetchall()

        if not top_artists:
            return []
        
        artist_name = [row['artist'] for row in top_artists]

        model = GenerativeModel("gemini-2.5-flash")
        prompt = """
        あなたは音楽おすすめAIです。ユーザーのお気に入りの曲名とアーティスト名を入力するので
        以下に対応する形でおすすめの音楽を出力してください。

        {
            "artist" : アーティスト名
            "music" : 音楽
            "url" : 曲のUrl
            "reason" : おすすめの理由
        }

        """
        response = model.generate_content(prompt)

        
        

if __name__ == "__main__":
    if len(sys.argv) > 1:
        uid = sys.argv[1]
    else:
        uid = "0"
    recommendations = get_recommendations(uid)

    print(json.dumps(recommendations, ensure_ascii=False))