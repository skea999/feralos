config:
  environment.DISPLAY: :0
  raw.idmap: both 1000 1000
  user.user-data: |
    #cloud-config
    runcmd:
      - 'apt-get update'
description: GUI LXD profile
devices:
  PASocket:
    path: /tmp/.pulse-native
    source: /run/user/1000/pulse/native
    type: disk
  X11-unix:
    path: /tmp/.X11-unix
    source: /tmp/.X11-unix
    type: disk
  mygpu:
    type: gpu
name: guiapps