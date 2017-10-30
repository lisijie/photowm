# 照片加水印

批量对照片进行尺寸调整，并加上拍摄时间、拍摄地点水印。LBS API使用腾讯的，不太稳定，且每秒限制4次调用。

## 使用

```php
$ go build
$ ./photowm -path=/data/photo -out=/data/photo-out -width=2000
```