FROM golang:1.22-bookworm AS builder
WORKDIR /root
COPY . .
RUN go build -o x-ui main.go


FROM debian:12-slim
RUN apt-get update && apt-get install -y --no-install-recommends -y ca-certificates libcap2-bin \
    && apt-get clean && rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# 以非 root 用户运行主进程。通过 cap_net_bind_service 能力支持绑定 <1024 端口（如 443），
# 避免容器内整体以 root 运行。
# 注意：容器内不创建 xray 系统用户，因此 Xray 子进程会以 app 用户身份运行
# （xray/user_unix.go 在找不到 xray 用户时自动跳过降权）。容器隔离已提供
# 进程边界，无需在容器内再次降权。
RUN groupadd -r app && useradd -r -g app -d /home/app -m app

WORKDIR /home/app
COPY --from=builder /root/x-ui /home/app/x-ui
COPY bin/. /home/app/bin/.

RUN setcap 'cap_net_bind_service=+ep' /home/app/x-ui \
    && mkdir -p /etc/x-ui \
    && chown -R app:app /home/app /etc/x-ui

VOLUME [ "/etc/x-ui" ]

USER app
CMD [ "./x-ui" ]
