FROM python:3.8-alpine

COPY requirements.txt main.py /patcher/
WORKDIR /patcher

RUN pip3 install -r requirements.txt

ENTRYPOINT ["python3","main.py"]
