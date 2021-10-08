# Amazon Lightsail CLI Extensions

This project is the source code of `lightsailctl`, a tool that
augments [Amazon Lightsail features in AWS CLI.][lscli]

## Usage

`lightsailctl` is executed **automatically** by AWS CLI when certain
subcommands are used, such as `aws lightsail push-container-image`.

```sh
$ lightsailctl --plugin -h

Usage of `lightsailctl --plugin`:
  --input payload
        plugin payload
  --input-stdin
        receive plugin payload on stdin
```

## Installing

### Homebrew 🍻

```sh
brew install aws/tap/lightsailctl
```

### From Source

`lightsailctl` is written in Go, so please [install Go.][getgo]

If all you want is to install `lightsailctl` binary, then do the following:

```sh
go install github.com/aws/lightsailctl@latest
```

> **Note:** the executable is installed in the directory named by the `GOBIN`
> environment variable, which defaults to `$GOPATH/bin` or `$HOME/go/bin` if
> the `GOPATH` environment variable is not set (on Windows that is `%GOPATH%\bin`
> or `%USERPROFILE%\go\bin` respectively).

Keep reading if you want to work with `lightsailctl` source code locally.

After you clone this repo and open your terminal app in it, you'll be
able to test and build this code like so:

```sh
go test ./...
go install ./...
```

## Under The Hood

Let's consider this command and see what actually happens:

```sh
aws lightsail push-container-image \
 --service-name hello \
 --image hello-world:latest \
 --label www
```

The above command pushes a local container image with tag
`hello-world:latest` to make it available in Lightsail container
service deployments for service `hello`.

This container image pushing logic requires a number of steps that are
outsourced from AWS CLI to `lightsailctl`.

Here's a shell invocation of `ligtsailctl` that approximates what AWS
CLI does when the command above is invoked:

```sh
$ echo '{
  "inputVersion": "1",
  "operation": "PushContainerImage",
  "payload": {
    "service": "hello",
    "label":   "www",
    "image":   "hello-world:latest"
  }
}' | lightsailctl --plugin --input-stdin

85fcec7ef3ef: Layer already exists 
3e5288f7a70f: Layer already exists 
56bc37de0858: Layer already exists 
1c91bf69a08b: Layer already exists 
cb42413394c4: Layer already exists 
Digest: sha256:0b159cd1ee1203dad901967ac55eee18c24da84ba3be384690304be93538bea8
Image "hello-world:latest" registered.
Refer to this image as ":hello.www.73" in deployments.
```

## Security Disclosures

See [CONTRIBUTING.md](CONTRIBUTING.md#security-issue-notifications) for
more information.

## Giving Feedback and Contributing

Aside from the security feedback covered above, do you have any
feedback, bug reports, questions or feature ideas?

You are welcome to write up an [issue][issue] for us.

Please read about [Contributing Guidelines.](CONTRIBUTING.md)

## License

This project is licensed under the Apache-2.0 License.

[lscli]: https://docs.aws.amazon.com/cli/latest/reference/lightsail/index.html
[getgo]: https://golang.org/doc/install
[issue]: https://github.com/aws/lightsailctl/issues/new
