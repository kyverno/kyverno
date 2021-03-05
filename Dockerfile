FROM alpine:latest

ENV KYVERNO_VERSION 1.3.3

RUN adduser kyverno -D \
  && apk add curl git openssh \
  && git config --global url.ssh://git@github.com/.insteadOf https://github.com/

RUN  curl -L --output /tmp/kyverno-cli_v1.3.3_linux_x86_64.tar.gz https://github.com/kyverno/kyverno/releases/download/v1.3.3/kyverno-cli_v1.3.3_linux_x86_64.tar.gz \
  && echo "607bc44ce6dca62d8608fe9eda9a59cf164fa729f4131df6303336eb81686b99  /tmp/kyverno-cli_v1.3.3_linux_x86_64.tar.gz" | sha256sum -c \
  && tar -xvzf /tmp/kyverno-cli_v1.3.3_linux_x86_64.tar.gz -C /usr/local/bin \
  && chmod +x /usr/local/bin/kyverno \
  && mkdir ~/.ssh \
  && ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts

USER kyverno

WORKDIR /src
