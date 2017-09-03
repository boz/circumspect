FROM alpine

ADD ./circumspect-linux ./circumspect

ENTRYPOINT ["./circumspect"]
