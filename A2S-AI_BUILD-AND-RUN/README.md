
# A2S-AI (Build and Run)

```
git clone https://github.com/a2s-ai/A2S_crush.git

cd A2S_crush/

vi crush.json

daniel@MacBook A2S_crush % cat crush.json
{
  "providers": {
    "ollama": {
      "name": "vLLM",
      "base_url": "https://XXX-XXX-XXX-XXX/v1/",
      "type": "openai",
      "models": [
        {
          "name": "MiniMax-M2-AWQ",
          "id": "MiniMax-M2-AWQ",
          "context_window": 196608,
          "default_max_tokens": 8192
        }
      ]
    }
  }
}
daniel@MacBook A2S_crush %
```

```
cp Dockerfile ../Dockerfile
cp docker_build_and_run.sh ../docker_build_and_run.sh

cd ..
./docker_build_and_run.sh
```

![a2s-ai](crush.png)

# A2S GPU Server - New Setup (> 01.11.2025) - vLLM with MiniMax-M2-AWQ

* Ubuntu 24 LTS VM with 4 x NVIDIA RTX 6000A

## vLLM (Docker) Settings with 196K (full) Context

```
root@ai-ubuntu24gpu-large:/opt# cat run-vllm-max_QuantTrio_MiniMax-M2-AWQ.sh
#!/bin/sh

export HUGGING_FACE_HUB_TOKEN=hf_XXX-XXX-XXX
export CUDA_VISIBLE_DEVICES="0,1,2,3"

docker network create vllm-max

docker run \
       --name vllm-minimax \
       --network vllm-minimax \
       --gpus all \
       --runtime=nvidia \
       --ipc=host \
       --restart unless-stopped -d --init \
       -p 8000:8000 \
       -v /data/opt/vllm:/root/.cache/huggingface \
       vllm/vllm-openai:nightly \
         --model a2s-ai/MiniMax-M2-AWQ \
         --served-model-name MiniMax-M2-AWQ \
         --tensor-parallel-size 4 \
         --enable-auto-tool-choice \
         --tool-call-parser minimax_m2 \
         --reasoning-parser minimax_m2_append_think \
         --max-model-len 196608 \
         --trust-remote-code

# EOF
root@ai-ubuntu24gpu-large:/opt#
```

# A2S GPU Server - Old Setup (< 01.11.2025) - Ollama with qwen3:235b-a22b-thinking-2507-q4_K_M

* Ubuntu 24 LTS VM with 4 x NVIDIA RTX 6000A

## Ollama (Docker) Settings with 64K Context

```
root@ai-ubuntu24gpu-large:~# cat /opt/run-ollama-max.sh
#!/bin/sh

export HUGGING_FACE_HUB_TOKEN=hf_XXX-XXX-XXX
export CUDA_VISIBLE_DEVICES="0,1,2,3"

docker network create ollama-max

docker run \
       --name ollama-max \
       --network ollama-max \
       --gpus='"device=0,1,2,3"' \
       --runtime=nvidia \
       --shm-size=8g \
       -p 11434:11434 \
       --restart unless-stopped -d --init \
       -e OLLAMA_HOME=/ollama-data \
       -v /data/opt/ollama:/ollama-data \
       -v /data/opt/ollama:/root/.ollama \
       -e OLLAMA_KEEP_ALIVE=-1 \
       -e OLLAMA_MAX_LOADED_MODELS=1 \
       -e OLLAMA_NUM_PARALLEL=4 \
       -e OLLAMA_MAX_QUEUE=1 \
       -e OLLAMA_ORIGINS="*" \
       -e OLLAMA_DEBUG=0 \
       -e OLLAMA_NUM_GPU_LAYERS=9999 \
       -e OLLAMA_DISABLE_CPU=1 \
       -e OLLAMA_LOAD_TIMEOUT=600 \
       -e OLLAMA_CONTEXT_LENGTH=65536 \
       -e OLLAMA_FLASH_ATTENTION=1 \
       ollama/ollama:latest

#       -e OLLAMA_NOPRUNE=1 \

# EOF
root@ai-ubuntu24gpu-large:~#
```

