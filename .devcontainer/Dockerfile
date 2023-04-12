FROM golang:1.20

RUN apt update && apt install -y sudo git

ARG USERNAME=root
RUN echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

ENV DEVCONTAINER=true

COPY . .
