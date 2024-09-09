FROM ubuntu:24.04@sha256:8a37d68f4f73ebf3d4efafbcf66379bf3728902a8038616808f04e34a9ab63ee

RUN apt-get update && apt-get install -y sudo git curl apt-transport-https ca-certificates gnupg-agent software-properties-common
ARG USERNAME=root
RUN echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Install Golang
RUN ARCH="$(dpkg --print-architecture)"; \
    curl -LO https://dl.google.com/go/go1.21.3.linux-$ARCH.tar.gz \
    && tar -C /usr/local -xzf go1.21.3.linux-$ARCH.tar.gz \
    && rm go1.21.3.linux-$ARCH.tar.gz \
    && echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile

# Install Docker
# Install Docker
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
RUN echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
    $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
RUN apt-get update && apt-get install -y docker-ce docker-ce-cli containerd.io

# Install kubectl and Minikube
RUN ARCH="$(dpkg --print-architecture)"; \
    curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/$ARCH/kubectl \
    && chmod +x kubectl && mv kubectl /usr/local/bin/ \
    && curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-$ARCH \
    && install minikube-linux-$ARCH /usr/local/bin/minikube \
    && minikube config set driver docker

# Expose ports for Minikube and Docker
EXPOSE 22 80 2375 8443

CMD ["/bin/bash"]
