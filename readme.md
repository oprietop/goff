# goff
Naive filefactory downloader

This program will download files from filefactory just retrying on each link until it gets a download slot, so no high speeds and resume.

It does the job if you have to download many files and you are not in a hurry.

## Usage:
Just create a `urls.txt` with all the links, one per line and execute the program.

## Notes
* Program will delete completed downloads from the urls.txt file

* Sometimes i got corrupted files after download so i recommend to make a backup of the urls.txt file before downloading.

## Todo
* Find if there's a way to checksum the files after download
