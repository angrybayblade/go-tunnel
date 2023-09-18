FROM golang:1.21.1

COPY . /root/

WORKDIR /root/

RUN go build

RUN chmod +x ./tunnel

RUN mv ./tunnel /usr/bin

CMD [ "tunnel" ]

ENTRYPOINT [ "tunnel" ]