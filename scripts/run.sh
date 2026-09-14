#!/bin/bash

BASE_DIR="/data/kvm-manager"
SERVER_NAME="kvm-manager"
PID_FILE="$BASE_DIR/app.pid"
LOG_FILE="$BASE_DIR/app.log"
APP="$BASE_DIR/kvm-manager"

PID=$(cat $PID_FILE 2>/dev/null)

log_message() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $1" >> "$LOG_FILE"
}

case $1 in
"start")
    # 检查 PID 文件中的进程是否真的存在
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo -e "\033[33m$SERVER_NAME 服务已运行 (PID: $PID)！\033[0m"
        exit 0
    fi

    # PID 文件存在，但进程已经不存在，清理旧 PID
    echo -n > "$PID_FILE"
    echo "正在启动 $SERVER_NAME..."

    cd "$BASE_DIR" || exit 1

    # 记录启动时间
    log_message "========== 启动 $SERVER_NAME =========="
    nohup "$APP" >> "$LOG_FILE" 2>&1 &

    NEW_PID=$!
    echo "$NEW_PID" > "$PID_FILE"
    sleep 1

    if kill -0 "$NEW_PID" 2>/dev/null; then
        echo -e "\033[32m$SERVER_NAME 服务启动成功！(PID: $NEW_PID)\033[0m"
    else
        echo -e "\033[31m$SERVER_NAME 服务启动失败，请检查日志：$LOG_FILE\033[0m"
        echo -n > "$PID_FILE"
    fi
;;

"stop")
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo "正在停止 $SERVER_NAME..."
        log_message "========== 停止 $SERVER_NAME，PID: $PID =========="

        kill "$PID" 2>/dev/null

        # 等待进程退出
        for i in {1..10}; do
            ! kill -0 "$PID" 2>/dev/null && break
            sleep 1
        done

        # 如果还没退出，再强制杀掉
        if kill -0 "$PID" 2>/dev/null; then
            echo "进程未正常退出，执行强制停止..."
            kill -9 "$PID" 2>/dev/null
        fi

        if ! kill -0 "$PID" 2>/dev/null; then
            echo -n > "$PID_FILE"
            echo -e "\033[32m$SERVER_NAME 服务停止成功！\033[0m"
        else
            echo -e "\033[31m$SERVER_NAME 服务停止失败！\033[0m"
        fi
    else
        echo -e "\033[33m$SERVER_NAME 服务未运行！\033[0m"
        echo -n > "$PID_FILE"
    fi
;;

"restart")
    echo "正在重启 $SERVER_NAME..."
    log_message "========== 重启 $SERVER_NAME =========="

    # 如果正在运行，先停止
    if [ -n "$PID" ] && kill -0 "$PID" 2>/dev/null; then
        echo "正在停止旧进程 (PID: $PID)..."
        kill "$PID" 2>/dev/null

        for i in {1..10}; do
            ! kill -0 "$PID" 2>/dev/null && break
            sleep 1
        done

        if kill -0 "$PID" 2>/dev/null; then
            echo "旧进程未正常退出，执行强制停止..."
            kill -9 "$PID" 2>/dev/null
            sleep 1
        fi
    fi
    
    echo -n > "$PID_FILE"
    cd "$BASE_DIR" || exit 1
    nohup "$APP" >> "$LOG_FILE" 2>&1 &

    NEW_PID=$!
    echo "$NEW_PID" > "$PID_FILE"
    sleep 1

    if kill -0 "$NEW_PID" 2>/dev/null; then
        echo -e "\033[32m$SERVER_NAME 服务重启成功！(PID: $NEW_PID)\033[0m"
    else
        echo -e "\033[31m$SERVER_NAME 服务重启失败，请检查日志：$LOG_FILE\033[0m"
        echo -n > "$PID_FILE"
    fi
;;

*)
    echo "请输入参数：start | stop | restart"
;;
esac