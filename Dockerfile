FROM golang:latest

WORKDIR /app

COPY . .

RUN go get gopkg.in/yaml.v2

RUN go build -o /professor-pundit

CMD ["/professor-pundit"]
