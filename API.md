# Utopia 节点代理 API 文档

本文档详细介绍了 Utopia 节点代理提供的 API 端点。

## 认证

所有 `/api/v1` 路由都受认证保护。客户端必须在 HTTP 请求的 `Authorization` 头中提供一个 Bearer Token。

**请求头示例:**
```
Authorization: Bearer <your_auth_token>
```

如果认证失败，API 将返回 `401 Unauthorized` 状态码。

---

## API 端点

### 1. 容器管理

#### 1.1 创建容器

*   **方法:** `POST`
*   **路径:** `/api/v1/containers`
*   **功能:** 创建并启动一个新的 Docker 容器。
*   **请求头:**
    *   `Authorization: Bearer <your_auth_token>`
*   **请求体 (JSON):**
    ```json
    {
      "claim_id": "string",
      "image": "string",
      "gpu_count": "integer",
      "port_mappings": [
        {
          "host_port": "integer",
          "container_port": "integer",
          "protocol": "string"
        }
      ],
      "env_vars": [
        "string"
      ],
      "command": [
        "string"
      ],
      "working_dir": "string",
      "volumes": {
        "string": "string"
      }
    }
    ```
*   **成功响应 (201 Created):**
    ```json
    {
      "container_id": "string"
    }
    ```

#### 1.2 删除容器

*   **方法:** `DELETE`
*   **路径:** `/api/v1/containers/:id`
*   **功能:** 停止并删除指定的容器。
*   **请求头:**
    *   `Authorization: Bearer <your_auth_token>`
*   **成功响应 (204 No Content):** 无响应体。

#### 1.3 列出所有容器

*   **方法:** `GET`
*   **路径:** `/api/v1/containers`
*   **功能:** 获取由代理管理的所有容器的列表。
*   **请求头:**
    *   `Authorization: Bearer <your_auth_token>`
*   **成功响应 (200 OK):**
    ```json
    [
      {
        "id": "string",
        "claim_id": "string",
        "image": "string",
        "status": "string",
        "gpu_ids": [
          "integer"
        ],
        "ports": {
          "string": "string"
        },
        "created": "integer",
        "started": "integer",
        "labels": {
          "string": "string"
        }
      }
    ]
    ```

#### 1.4 获取单个容器信息

*   **方法:** `GET`
*   **路径:** `/api/v1/containers/:id`
*   **功能:** 获取指定容器的详细信息。
*   **请求头:**
    *   `Authorization: Bearer <your_auth_token>`
*   **成功响应 (200 OK):**
    ```json
    {
      "id": "string",
      "claim_id": "string",
      "image": "string",
      "status": "string",
      "gpu_ids": [
        "integer"
      ],
      "ports": {
        "string": "string"
      },
      "created": "integer",
      "started": "integer",
      "labels": {
        "string": "string"
      }
    }
    ```

### 2. 系统指标

#### 2.1 获取系统指标

*   **方法:** `GET`
*   **路径:** `/api/v1/metrics`
*   **功能:** 获取节点的系统和 GPU 指标。
*   **请求头:**
    *   `Authorization: Bearer <your_auth_token>`
*   **成功响应 (200 OK):**
    ```json
    {
      "node_id": "string",
      "cpu_usage_percent": "number",
      "memory_usage_percent": "number",
      "gpus": [
        {
          "id": "integer",
          "temperature_c": "integer",
          "memory_total_mb": "integer",
          "memory_used_mb": "integer",
          "name": "string",
          "uuid": "string",
          "busy": "boolean",
          "usage_percent": "number"
        }
      ],
      "system": {
        "cpu_usage_percent": "number",
        "memory_usage_percent": "number",
        "memory_total_mb": "integer",
        "memory_used_mb": "integer",
        "disk_usage_percent": "number",
        "load_average": "number",
        "uptime": "integer"
      }
    }
    ```

### 3. 健康检查

#### 3.1 健康检查

*   **方法:** `GET`
*   **路径:** `/health`
*   **功能:** 检查节点代理的健康状况。此端点**不需要**认证。
*   **成功响应 (200 OK):**
    ```json
    {
      "status": "healthy",
      "timestamp": "string"
    }