# 运行时镜像。二进制由 GoReleaser 构建好后放进上下文，这里只做打包。
#
# 刻意不在这里 go build：那样镜像里的二进制与 Releases 里的会来自两次
# 不同的构建，ldflags 注入的版本信息也对不上，而「我下的二进制和镜像里
# 表现不一样」是极难排查的一类问题。
#
# 用 alpine 而非 scratch/distroless：这是自托管工具，用户需要能
# docker exec 进来看「为什么连不上 B 站」。没有 shell 的镜像排障时
# 只能干瞪眼，多出来的 7 MB 在一个带 PostgreSQL 的部署里可以忽略。
FROM alpine:3.20

# ca-certificates: 访问 B 站 HTTPS 接口必需
# tzdata:          定时规则（cron）按本地时区算，没有它一律是 UTC
RUN apk add --no-cache ca-certificates tzdata

# 非 root 运行。magicd 不需要任何特权，也不监听 1024 以下端口。
RUN addgroup -g 10001 magicd && adduser -D -u 10001 -G magicd magicd

COPY magicd /usr/local/bin/magicd

USER magicd:magicd

# 时区默认跟随宿主的 TZ 环境变量；未设置时是 UTC。
# 定时规则写的是几点几分，时区错了会整体偏移，部署时记得设。
ENV TZ=UTC

ENTRYPOINT ["/usr/local/bin/magicd"]
CMD ["run"]
