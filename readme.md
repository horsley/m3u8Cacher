# m3u8cacher

这个小工具的任务很简单，解析 m3u8文件里面的视频分片，并下载，本地再起一个 http server 来提供服务，相当于一个本地中转缓存，对于网速不给力或者盒子不给力的同学友好（直接在线播卡成 ppt。

## 用法
主要看命令行选项
```
Usage of ./m3u8Cacher:
  -d, --download string   m3u8 download address
  -f, --force-download    force download even if file already exist
  -l, --listen string     http server listen address (default ":8000")
  -o, --output string     output(download) directory (default "out")
  -t, --thread int        download thread limit (default 10)
  -w, --use-working-dir   use working directory instead of executable directory
```