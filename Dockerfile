FROM golang:latest

WORKDIR /app

COPY . .

RUN go build -o /professor-pundit

CMD ["/professor-pundit"]
