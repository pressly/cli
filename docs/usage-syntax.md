# Usage Syntax Conventions

These conventions follow the [POSIX utility argument syntax][def] and are widely used by tools like
`docker`, `kubectl`, and `git`.

| Syntax         | Description               |
| -------------- | ------------------------- |
| `<required>`   | Required argument         |
| `[optional]`   | Optional argument         |
| `<arg>...`     | One or more arguments     |
| `[arg]...`     | Zero or more arguments    |
| `(a\|b)`       | Must choose one of a or b |
| `[-f <value>]` | Optional flag with value  |
| `-f <value>`   | Required flag with value  |

## Examples

```
# Positional arguments
cp <source>... <dest>

# Mix of required flag and optional positional args
build -t <tag> [config]...

# Subcommand with optional flag
app (start|stop) [-n <name>]

# Repeatable optional flag
search [--exclude <pattern>]... <path>
```

[def]: https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/V1_chap12.html
