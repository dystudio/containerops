## BUILD

```bash
docker build -t containerops/green .
```

## TEST

```bash
docker run --rm -e CO_DATA='git-url=https://github.com/minhhh/regex.git entry-path=.' containerops/green
```

## TEST with deference python version

`version` is one of `python`, `python2`, `python3`, `py3k`.
default is `py3k`

```bash
docker run --rm -e CO_DATA='git-url=https://github.com/minhhh/regex.git entry-path=. version=python' containerops/green
```