FROM ubuntu:latest@sha256:aabed3296a3d45cede1dc866a24476c4d7e093aa806263c27ddaadbdce3c1054

RUN apt-get update && apt-get install -y sudo git curl apt-transport-https ca-certificates gnupg-agent software-properties-common
ARG USERNAME=root
RUN echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

# Install Golang
RUN curl -LO https://dl.google.com/go/go1.20.linux-amd64.tar.gz \
    && tar -C /usr/local -xzf go1.20.linux-amd64.tar.gz \
    && rm go1.20.linux-amd64.tar.gz \
    && echo 'export PATH=$PATH:/usr/local/go/bin' >> /etc/profile

# Install Docker
# Install Docker
RUN curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
RUN echo \
  "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu \
  $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null
RUN apt-get update && apt-get install -y docker-ce docker-ce-cli containerd.io

# Install kubectl and Minikube
RUN curl -LO https://storage.googleapis.com/kubernetes-release/release/$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl \
    && chmod +x kubectl && mv kubectl /usr/local/bin/ \
    && curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64 \
    && install minikube-linux-amd64 /usr/local/bin/minikube \
    && minikube config set driver docker

# Expose ports for Minikube and Docker
EXPOSE 22 80 2375 8443

CMD ["/bin/bash"]
