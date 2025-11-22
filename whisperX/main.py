from flask import Flask, request, Response
import whisperx
import json
import numpy as np

app = Flask(__name__)

print("Model loading...")
# model = whisperx.load_model("medium", "cuda", compute_type="float16")
model = whisperx.load_model("medium", "cpu", compute_type="int8")
print("Model loaded.")

@app.route('/', methods=['POST'])
def transcription():
    try:
        raw_data = request.data

        if not raw_data:
            return Response(response="Audio data is missing", status=400)

        audio_array = np.frombuffer(raw_data, dtype=np.int16)
        audio_array = audio_array.astype(np.float32) / 32768.0

        print(f"Received audio chunk size: {len(audio_array)} samples")

        # 4. Transkripsiyon
        result = model.transcribe(audio_array, language="tr", batch_size=16)
        print(result)

    except Exception as e:
        print(f"Error: {e}")
        return Response(response=f"An error occurred: {str(e)}", status=500)

    return Response(json.dumps(result, ensure_ascii=False), content_type="application/json")


if __name__ == '__main__':
    app.run(debug=True, use_reloader=False, host="0.0.0.0", port=5000)