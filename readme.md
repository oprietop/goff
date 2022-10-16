# goff
Naive filefactory downloader

This program will download files from filefactory just retrying on each link until it gets a download slot, so no high sspeeds and resume.

It does the job for me when i have to download many files and i'm not in a hurry to have them.

## Usage:
Just create a `urls.txt` with all the links, one per line and execute the program.

## Notes
* Program will delete completed downloads from the urls.txt file

* Sometimes i got corrupted files after download so i recommend to make a backup of the urls.txt file before downloading.

## Todo
* Find if there's a way to checksum the files after download
