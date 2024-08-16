FROM python:3.13.0rc1-alpine3.19

COPY requirements.txt main.py /patcher/
WORKDIR /patcher

RUN pip3 install -r requirements.txt

ENTRYPOINT ["python3","main.py"]
