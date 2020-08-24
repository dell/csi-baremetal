FROM ubuntu:20.04

RUN apt-get update \
  && apt-get install -y python3-pip python3-dev \
  && cd /usr/local/bin \
  && ln -s /usr/bin/python3 python \
  && pip3 install --upgrade pip

COPY requirements.txt /patcher/
WORKDIR /patcher
RUN pip3 install -r requirements.txt

COPY main.py /patcher/

ENTRYPOINT ["python3","main.py"]
