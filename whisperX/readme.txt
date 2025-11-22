main.py dosyasında aşşağıdaki kod filenotfound hatası verirse muhtemelen ffmpeg yüklü olmadığı için hata alıyorsunuz.

process = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE
        )

-----------------------------------------------------------------------------------------------------------------------