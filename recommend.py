import sys
import json
import mysql.connector
import os
import google.generativeai as genai

api_key = os.getenv("GEMINI_API_KEY")
genai.configure(api_key=api_key)

def get_recommendations(user_id):
    try:
        connection = mysql.connector.connect(
            host="mysql_db",
            user="root",
            password = "password",
            database = "music_db"
        )
        cursor = connection.cursor(dictionary = True)

        artist_query = """
            SELECT artist, COUNT(*) as play_count
            FROM raw_watch_logs
            WHERE user_id = %s
            GROUP BY artist
            ORDER BY play_count DESC
            LIMIT 3
            """
        cursor.execute(artist_query,(user_id,))
        top_artists = cursor.fetchall()

        if not top_artists:
            print("データベースから取得失敗")
            return []
        
        artist_name = [row['artist'] for row in top_artists]
        artist_name_str = ",".join(artist_name)

        model = genai.GenerativeModel("gemini-2.5-flash")
        prompt = f"""
        あなたは音楽おすすめAIです。ユーザーがよく聴くアーティストに基づき、次に聴くべきおすすめの音楽を【3曲】提案してください。
        
        ユーザーがよく聴くアーティスト: {artist_names_str}

        【出力フォーマット】
        必ず、以下の構造を持つ有効なJSON配列（List）オブジェクトのみを出力してください。
        余計な挨拶、解説、バックコォート(```json)などは一切含めず、純粋なJSON文字列だけを返してください。

        [
            {{
                "artist" : "アーティスト名",
                "music" : "曲名",
                "url" : "曲のURL（わからない場合は空文字列）",
                "reason" : "おすすめする具体的な理由"
            }}
        ]
        """
        response = model.generate_content(prompt)
        response_text = response.text.strip()
        print(f"geminiの回答:\n{response_text}",file = sys.stderr)
        try:
            recommended_data = json.loads(response_text)

            if not isinstance(recommended_data, list):
                print("[error] GeminiのレスポンスがJSON配列ではありません")
                return []
            required_keys = {"artist","music", "url" , "reason"}
            valid_recommendation = []

            for item in recommended_data:
                if isinstance(item,dict) and required_keys.issubset(item.keys()):
                    valid_recommendation.append({
                    "artist" : item["artist"],
                    "music" : item["music"],
                    "url": item["url"],
                    "reason": item["reason"]
                    })
                else:
                    print(f"不正なフォーマットがありました:{item}",file = sys.stderr)
            return valid_recommendation
        except json.JSONDecodeError as je:
            print(f"⚠️ [Error] Geminiの出力をJSONとして読み込めませんでした。: {je}", file=sys.stderr)
            print(f"元の出力: {response_text}", file=sys.stderr)
            return []


    except Exception as e:
        # データベース接続エラーやAPIキー不足など、予期せぬ重大エラー時の防衛線
        print(f"⚠️ [Fatal Error] システムエラーが発生しました: {e}", file=sys.stderr)
        return []



if __name__ == "__main__":
    if len(sys.argv) > 1:
        try:
            uid = int(sys.argv[1])
        except ValueError:
            uid = 1
    else:
        uid = 1

    recommendations = get_recommendations(uid)
    print(json.dumps(recommendations, ensure_ascii=False))
    