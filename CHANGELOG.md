# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2021-07-29

### Changed

- Expose hub API via kube-apiserver
- Change to use RBAC in agent instead of front-proxy cert

## [0.2.0] - 2021-06-28

### Added

- Support for minikube and kind member clusters
- Support for multiple agent replicas on one member cluster
- Support for multiple hub replicas

## Changed

- Rename external_addr flag to public_addr

## [0.1.0] - 2021-06-15

### Added

- Tunnelled proxy between hub and agent
- Kubernetes manifests
