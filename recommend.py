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
            password="password",
            database="music_db"
        )
        cursor = connection.cursor(dictionary=True)

        artist_query = """
            SELECT artist, COUNT(*) as play_count
            FROM raw_watch_logs
            WHERE user_id = %s
            GROUP BY artist
            ORDER BY play_count DESC
            LIMIT 3
            """
        cursor.execute(artist_query, (user_id,))
        top_artists = cursor.fetchall()

        cursor.close()
        connection.close()

        if not top_artists:
            print("⚠️ [Debug] データベースからトップアーティストの取得に失敗、または0件です", file=sys.stderr)
            return []
        
        artist_name = [row['artist'] for row in top_artists]
        # ⭕ 変数名を artist_names_str に統一してプロンプトと合わせる
        artist_names_str = ",".join(artist_name)

        model = genai.GenerativeModel(
            "gemini-2.5-flash",
            generation_config={"response_mime_type": "application/json"}
        )

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

        if response_text.startswith("```"):
            lines = response_text.splitlines()
            if lines[0].startswith("```"):
                lines = lines[1:]
            if lines[-1].startswith("```"):
                lines = lines[:-1]
            response_text = "\n".join(lines).strip()

        print(f"🤖 [Debug] geminiの回答:\n{response_text}", file=sys.stderr)
        
        try:
            recommended_data = json.loads(response_text)

            if not isinstance(recommended_data, list):
                print("[error] GeminiのレスポンスがJSON配列ではありません", file=sys.stderr)
                return []
            
            required_keys = {"artist", "music", "url", "reason"}
            valid_recommendation = []

            for item in recommended_data:
                if isinstance(item, dict) and required_keys.issubset(item.keys()):
                    valid_recommendation.append({
                        "artist": item["artist"],
                        "music": item["music"],
                        "url": item["url"],
                        "reason": item["reason"]
                    })
                else:
                    print(f"不正なフォーマットがありました:{item}", file=sys.stderr)
            return valid_recommendation
            
        except json.JSONDecodeError as je:
            print(f"⚠️ [Error] Geminiの出力をJSONとして読み込めませんでした。: {je}", file=sys.stderr)
            print(f"元の出力: {response_text}", file=sys.stderr)
            return []

    except Exception as e:
        # ⭕ 重要: どこでバグが起きても追えるように原因（e）を stderr に出力するように強化！
        print(f"⚠️ [Fatal Error] システムエラーが発生しました: {e}", file=sys.stderr)
        import traceback
        traceback.print_exc(file=sys.stderr) # スタックトレースも出す
        return []

if __name__ == "__main__":
    print("🔥 [System Check] Pythonスクリプトが正常に起動しました！", file=sys.stderr)
    print(f"🔥 [System Check] 受け取った引数(sys.argv): {sys.argv}", file=sys.stderr)

    if len(sys.argv) > 1:
        try:
            uid = int(sys.argv[1])
        except ValueError:
            print(f"⚠️ [System Check] 引数 '{sys.argv[1]}' をintに変換できませんでした。デフォルトの1を使用します。", file=sys.stderr)
            uid = 1
    else:
        uid = 1

    print(f"🔥 [System Check] 最終的に関数に渡すユーザーID: {uid}", file=sys.stderr)
    
    recommendations = get_recommendations(uid)
    print(json.dumps(recommendations, ensure_ascii=False))