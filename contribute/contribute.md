# Contribute README
Thanks for your contribution, you can follow these step to run this repo and debug it.
## Run demo
1. clone this repo to your work dir.
    ```bash
    git clone https://github.com/mark3labs/mcphost.git
    ```

2. enter the `contribute` dir.
    ```bash
    cd mcphost/contribute
    ```

3. run `build.sh` to build your binary file.
   ```bash
      ./build.sh
   ```

4. open `boost.sh` file and fill your model info in.
   ```bash
      cat boost.sh
      vi boost.sh
   ```

5. run `boost.sh` to run your mcphost, if you don't want run it in debug model, you can delete the `--debug` flag in `boost.sh`.
   ```bash
      ./boost.sh
   ```

## Contribute your code
just write your code and push it.
