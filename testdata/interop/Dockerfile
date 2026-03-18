FROM python:3.12-slim

RUN pip install --no-cache-dir pycdr2

COPY generate.py /work/generate.py
WORKDIR /work

CMD ["python", "generate.py"]
