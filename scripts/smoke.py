"""Integration checks against a running full Docker installation.
Usage: python3 scripts/smoke.py http://localhost:8080
"""
import io
import json
import struct
import sys
import urllib.request
import uuid
import wave
import zipfile
import zlib

base = sys.argv[1] if len(sys.argv) > 1 else 'http://localhost:8080'

def convert(name, target, content):
    boundary = uuid.uuid4().hex
    body = (f'--{boundary}\r\nContent-Disposition: form-data; name="format"\r\n\r\n{target}\r\n'
            f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{name}"\r\n'
            'Content-Type: application/octet-stream\r\n\r\n').encode()
    body += content + f'\r\n--{boundary}--\r\n'.encode()
    request = urllib.request.Request(base + '/api/convert', data=body,
        headers={'Content-Type': 'multipart/form-data; boundary=' + boundary})
    with urllib.request.urlopen(request, timeout=190) as response:
        result = response.read()
        assert result and response.headers.get('Content-Disposition')
    print(f'PASS {name} -> {target}', flush=True)
    return result

assert json.load(urllib.request.urlopen(base + '/api/health'))['status'] == 'ok'
assert json.loads(convert('data.csv', 'json', b'name,count\nexample,42\n'))[0]['count'] == '42'
assert b'example' in convert('data.json', 'csv', b'[{"name":"example"}]')

def chunk(kind, payload):
    return struct.pack('!I', len(payload)) + kind + payload + struct.pack('!I', zlib.crc32(kind + payload))
png = b'\x89PNG\r\n\x1a\n' + chunk(b'IHDR', struct.pack('!2I5B', 2, 2, 8, 2, 0, 0, 0))
png += chunk(b'IDAT', zlib.compress(b'\x00\xff\x00\x00\x00\xff\x00' * 2)) + chunk(b'IEND', b'')
assert convert('image.png', 'jpg', png).startswith(b'\xff\xd8')
webp = convert('image.png', 'webp', png)
assert convert('image.webp', 'png', webp).startswith(b'\x89PNG')
docx = convert('document.txt', 'docx', b'Hello from Forma.\n')
assert zipfile.is_zipfile(io.BytesIO(docx))
pdf = convert('document.docx', 'pdf', docx)
assert pdf.startswith(b'%PDF')
assert b'Hello from Forma' in convert('document.pdf', 'txt', pdf)
assert convert('document.pdf', 'png', pdf).startswith(b'\x89PNG')
# Two-page PDF checks that all pages are returned as a ZIP archive.
objects = [
    b'<< /Type /Catalog /Pages 2 0 R >>',
    b'<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>',
    b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << >> >>',
    b'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << >> >>',
]
multipage = b'%PDF-1.4\n'
offsets = [0]
for index, obj in enumerate(objects, 1):
    offsets.append(len(multipage))
    multipage += f'{index} 0 obj\n'.encode() + obj + b'\nendobj\n'
xref = len(multipage)
multipage += f'xref\n0 {len(offsets)}\n0000000000 65535 f \n'.encode()
for offset in offsets[1:]:
    multipage += f'{offset:010} 00000 n \n'.encode()
multipage += f'trailer\n<< /Size {len(offsets)} /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF'.encode()
archive = convert('pages.pdf', 'jpg', multipage)
with zipfile.ZipFile(io.BytesIO(archive)) as pages:
    assert len(pages.namelist()) == 2
    assert all(pages.read(name).startswith(b'\xff\xd8') for name in pages.namelist())
assert b'<h1' in convert('document.md', 'html', b'# Hello\n\nA document.')
audio = io.BytesIO()
with wave.open(audio, 'wb') as wav:
    wav.setnchannels(1)
    wav.setsampwidth(2)
    wav.setframerate(8000)
    wav.writeframes(b'\x00\x00' * 8000)
mp3 = convert('audio.wav', 'mp3', audio.getvalue())
assert convert('audio.mp3', 'wav', mp3).startswith(b'RIFF')
print('All integration checks passed.')
