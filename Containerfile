FROM registry.access.redhat.com/ubi8/ubi:latest AS deploy
ENV agv_version=v0.8.1
RUN dnf install -y \
    bash \
    bind-utils \
    curl \
    findutils \
    gcc \
    git \
    jq \
    nc \
    net-tools \
    libcurl-devel \
    libxml2-devel \
    openssl \
    openssl-devel \
    openssl \
    python39 \
    python39-pip \
    rsync \
    tar \
    unzip \
    vim \
    wget \
    && dnf clean all

# Install agnosticv CLI
RUN curl --silent --location -o /usr/bin/agnosticv.${agv_version} \
  https://github.com/redhat-cop/agnosticv/releases/download/${agv_version}/agnosticv_linux_amd64
RUN chmod +x /usr/bin/agnosticv.${agv_version}
RUN ln -s /usr/bin/agnosticv.${agv_version} /usr/bin/agnosticv

# Python

RUN alternatives --set python /usr/bin/python3.9 \
    && alternatives --set python3 /usr/bin/python3.9 \
    && alternatives --install /usr/bin/pip pip /usr/bin/pip3.9 1
RUN pip install --no-cache-dir --upgrade pip

RUN rm -rf /tmp/* /root/.cache /root/*
USER ${USER_UID}
CMD ["/bin/bash"]

ENV DESCRIPTION="Image for Admins containing AgnosticV CLI"
LABEL name="rhpds/agnosticv" \
      maintainer="Red Hat Demo Platform" \
      description="${DESCRIPTION}" \
      summary="${DESCRIPTION}"
