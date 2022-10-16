#!/bin/sh
# https://gist.github.com/asukakenji/f15ba7e588ac42795f421b48b8aede63
GOOS="linux windows freebsd"
GOARCH="386 amd64"

for gofile in `echo *go`; do
    echo "[$gofile]"
    for goos in $GOOS; do
        for goarch in $GOARCH; do
            outfile=${gofile%.*}_${goos}-${goarch}
            [ "$goos" == "windows" ] && outfile="${outfile}.exe"
            echo "> Compiling $outfile"
            GOOS=$goos GOARCH=$goarch go build -ldflags="-s -w" -o $outfile $gofile
            if `which upx > /dev/null`; then
                echo "> Compressing binaries"
                upx -9 --lzma $outfile
            fi
        done
    done
done
