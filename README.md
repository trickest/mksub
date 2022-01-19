mksub
-----
Make subdomains using a wordlist

Read a wordlist file and generate subdomains for given domain or list of domains.
Input from wordlist file is lowercased and unique words are processed. Additionally, wordlist can be
filtered using regex. 

```
Usage of mksub:
  -d string
        Input domain
  -df string
        Input domain file, one domain per line
  -l int
        Subdomain level to generate (default 1)
  -o string
        Output file (optional)
  -r string
        Regex to filter words from wordlist file
  -t int
        Maximum number of threads to be used (default 200)
  -w string
        Wordlist file
```

### Example

##### wordlist.txt
```
dev
DEV
*
foo.bar
prod
```
```shell script
> go run mksub.go -d example.com -l 2 -w input.txt -r "^[a-zA-Z0-9\.-_]+$"
dev.example.com
foo.bar.example.com
prod.example.com
foo.bar.dev.example.com
prod.dev.example.com
dev.dev.example.com
dev.foo.bar.example.com
foo.bar.foo.bar.example.com
prod.foo.bar.example.com
dev.prod.example.com
foo.bar.prod.example.com
prod.prod.example.com

```
