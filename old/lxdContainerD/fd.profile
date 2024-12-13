config: {}
description: ""
devices:
  PASocket:
    path: /tmp/.pulse-native
    source: /run/user/1000/pulse/native
    type: disk
  gpu:
    gid: "13"
    type: gpu
  wayland-socket:
    bind: container
    connect: unix:/run/user/1000/wayland-0
    gid: "1000"
    listen: unix:/mnt/wayland-socket
    security.gid: "1000"
    security.uid: "1000"
    type: proxy
    uid: "1000"
name: fd
