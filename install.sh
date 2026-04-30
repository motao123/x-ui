#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

cur_dir=$(pwd)
required_go_version="1.22.0"
install_go_version="1.22.12"

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}错误：${plain} 必须使用root用户运行此脚本！\n" && exit 1

# check os
if [[ -f /etc/redhat-release ]]; then
    release="centos"
elif cat /etc/issue | grep -Eqi "debian"; then
    release="debian"
elif cat /etc/issue | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /etc/issue | grep -Eqi "centos|red hat|redhat"; then
    release="centos"
elif cat /proc/version | grep -Eqi "debian"; then
    release="debian"
elif cat /proc/version | grep -Eqi "ubuntu"; then
    release="ubuntu"
elif cat /proc/version | grep -Eqi "centos|red hat|redhat"; then
    release="centos"
else
    echo -e "${red}未检测到系统版本，请联系脚本作者！${plain}\n" && exit 1
fi

arch=$(arch)

if [[ $arch == "x86_64" || $arch == "x64" || $arch == "amd64" ]]; then
    arch="amd64"
elif [[ $arch == "aarch64" || $arch == "arm64" ]]; then
    arch="arm64"
elif [[ $arch == "s390x" ]]; then
    arch="s390x"
else
    arch="amd64"
    echo -e "${red}检测架构失败，使用默认架构: ${arch}${plain}"
fi

echo "架构: ${arch}"

if [ $(getconf WORD_BIT) != '32' ] && [ $(getconf LONG_BIT) != '64' ]; then
    echo "本软件不支持 32 位系统(x86)，请使用 64 位系统(x86_64)，如果检测有误，请联系作者"
    exit -1
fi

os_version=""

# os version
if [[ -f /etc/os-release ]]; then
    os_version=$(awk -F'[= ."]' '/VERSION_ID/{print $3}' /etc/os-release)
fi
if [[ -z "$os_version" && -f /etc/lsb-release ]]; then
    os_version=$(awk -F'[= ."]+' '/DISTRIB_RELEASE/{print $2}' /etc/lsb-release)
fi

if [[ x"${release}" == x"centos" ]]; then
    if [[ ${os_version} -le 6 ]]; then
        echo -e "${red}请使用 CentOS 7 或更高版本的系统！${plain}\n" && exit 1
    fi
elif [[ x"${release}" == x"ubuntu" ]]; then
    if [[ ${os_version} -lt 16 ]]; then
        echo -e "${red}请使用 Ubuntu 16 或更高版本的系统！${plain}\n" && exit 1
    fi
elif [[ x"${release}" == x"debian" ]]; then
    if [[ ${os_version} -lt 8 ]]; then
        echo -e "${red}请使用 Debian 8 或更高版本的系统！${plain}\n" && exit 1
    fi
fi

install_base() {
    if [[ x"${release}" == x"centos" ]]; then
        yum install wget curl tar git gcc make -y
    else
        apt update
        apt install wget curl tar git gcc make -y
    fi
}

version_ge() {
    # Returns success when $1 >= $2. Versions are compared by sort -V when available.
    [[ "$1" == "$2" ]] && return 0
    [[ "$(printf '%s\n%s\n' "$2" "$1" | sort -V | head -n1)" == "$2" ]]
}

get_go_version() {
    go version 2>/dev/null | awk '{print $3}' | sed 's/^go//'
}

ensure_go_for_build() {
    current_go_version=$(get_go_version)
    if [[ -n "${current_go_version}" ]] && version_ge "${current_go_version}" "${required_go_version}"; then
        echo -e "${green}Go ${current_go_version} 满足源码构建要求${plain}"
        return 0
    fi

    if [[ -n "${current_go_version}" ]]; then
        echo -e "${yellow}当前 Go ${current_go_version} 低于项目要求 ${required_go_version}，安装官方 Go ${install_go_version}${plain}"
    else
        echo -e "${yellow}未检测到 Go，安装官方 Go ${install_go_version}${plain}"
    fi

    go_arch="${arch}"
    if [[ "${arch}" == "arm64" ]]; then
        go_arch="arm64"
    elif [[ "${arch}" == "amd64" ]]; then
        go_arch="amd64"
    else
        echo -e "${red}源码构建暂不支持当前架构自动安装 Go: ${arch}${plain}"
        return 1
    fi

    go_tar="/tmp/go${install_go_version}.linux-${go_arch}.tar.gz"
    if ! wget -q --show-progress -O "${go_tar}" "https://go.dev/dl/go${install_go_version}.linux-${go_arch}.tar.gz"; then
        echo -e "${red}下载 Go ${install_go_version} 失败，请检查网络${plain}"
        rm -f "${go_tar}"
        return 1
    fi
    rm -rf /usr/local/go
    if ! tar -C /usr/local -xzf "${go_tar}"; then
        echo -e "${red}解压 Go ${install_go_version} 失败，可能是磁盘空间不足${plain}"
        rm -f "${go_tar}"
        return 1
    fi
    rm -f "${go_tar}"
    export PATH="/usr/local/go/bin:${PATH}"

    current_go_version=$(get_go_version)
    if [[ -z "${current_go_version}" ]] || ! version_ge "${current_go_version}" "${required_go_version}"; then
        echo -e "${red}Go 安装后版本仍不满足要求，当前版本: ${current_go_version:-unknown}${plain}"
        return 1
    fi
    echo -e "${green}已启用 Go ${current_go_version}${plain}"
}

available_kb() {
    df -Pk "$1" 2>/dev/null | awk 'NR==2 {print $4}'
}

cleanup_stale_build_artifacts() {
    # 清理之前源码构建失败可能留下的临时目录、swap、下载包与 Go 缓存。
    # 旧版本脚本会使用默认 /root/go/pkg/mod 和 /root/.cache/go-build，低磁盘机器反复安装时容易被这些残留占满。
    swapoff /tmp/x-ui-build.swap >/dev/null 2>&1 || true
    rm -rf /tmp/x-ui-src /tmp/x-ui-build.swap /tmp/go${install_go_version}.linux-*.tar.gz
    rm -rf /root/.cache/go-build /root/go/pkg/mod/cache/download
    go clean -modcache -cache >/dev/null 2>&1 || true
}

install_from_source() {
    echo -e "${yellow}未找到可用 Release 包，改为从源码构建安装${plain}"

    cleanup_stale_build_artifacts

    ensure_go_for_build || exit 1

    tmp_free_kb=$(available_kb /tmp)
    if [[ -n "${tmp_free_kb}" && ${tmp_free_kb} -lt 1800000 ]]; then
        echo -e "${red}/tmp 可用空间不足，源码构建至少建议预留 1.8GB；当前约 $((tmp_free_kb / 1024))MB${plain}"
        echo -e "${yellow}请先清理磁盘空间，或优先发布/下载 Release 包避免在小磁盘机器上源码编译${plain}"
        exit 1
    fi

    # 源码构建会编译 go-sqlite3 / xray-core 等依赖，小内存 VPS 容易在 gcc 阶段被 OOM Killer 杀掉：
    #   gcc: fatal error: Killed signal terminated program as
    # 这里在未启用 swap 且内存较小时临时创建 swap，并限制 Go 并行编译，提升低配机器安装成功率。
    swap_created=""
    tmp_dir="/tmp/x-ui-src"
    build_gomodcache="${tmp_dir}/.gomodcache"
    build_gocache="${tmp_dir}/.gocache"

    cleanup_source_build() {
        if [[ -n "${swap_created}" ]]; then
            swapoff "${swap_created}" >/dev/null 2>&1 || true
            rm -f "${swap_created}"
        fi
        rm -rf "${tmp_dir}"
        rm -f /tmp/go${install_go_version}.linux-*.tar.gz
    }

    mem_total_kb=$(awk '/MemTotal/ {print $2}' /proc/meminfo 2>/dev/null)
    swap_total_kb=$(awk '/SwapTotal/ {print $2}' /proc/meminfo 2>/dev/null)
    if [[ -n "${mem_total_kb}" && ${mem_total_kb} -lt 1500000 && "${swap_total_kb:-0}" -eq 0 ]]; then
        tmp_free_kb=$(available_kb /tmp)
        swap_mb=0
        if [[ -n "${tmp_free_kb}" && ${tmp_free_kb} -ge 5000000 ]]; then
            swap_mb=2048
        elif [[ -n "${tmp_free_kb}" && ${tmp_free_kb} -ge 2600000 ]]; then
            swap_mb=1024
        elif [[ -n "${tmp_free_kb}" && ${tmp_free_kb} -ge 1900000 ]]; then
            swap_mb=768
        fi

        if [[ ${swap_mb} -eq 0 ]]; then
            echo -e "${yellow}/tmp 可用空间不足，跳过临时 swap，避免依赖下载时磁盘被占满${plain}"
        else
            echo -e "${yellow}检测到内存较低且未启用 swap，创建临时 ${swap_mb}M swap 用于编译${plain}"
        fi

        if [[ ${swap_mb} -gt 0 ]] && (fallocate -l ${swap_mb}M /tmp/x-ui-build.swap 2>/dev/null || dd if=/dev/zero of=/tmp/x-ui-build.swap bs=1M count=${swap_mb}); then
            chmod 600 /tmp/x-ui-build.swap
            mkswap /tmp/x-ui-build.swap >/dev/null 2>&1 && swapon /tmp/x-ui-build.swap >/dev/null 2>&1 && swap_created="/tmp/x-ui-build.swap"
        fi
        if [[ -z "${swap_created}" ]]; then
            echo -e "${yellow}临时 swap 创建失败，将继续尝试低并行编译${plain}"
            rm -f /tmp/x-ui-build.swap
        fi
    fi

    rm -rf "${tmp_dir}"
    git clone --depth=1 https://github.com/motao123/x-ui.git "${tmp_dir}"
    if [[ $? -ne 0 ]]; then
        echo -e "${red}拉取源码失败，请检查服务器能否访问 Github${plain}"
        cleanup_source_build
        exit 1
    fi

    cd "${tmp_dir}"
    mkdir -p "${build_gomodcache}" "${build_gocache}"
    GOMODCACHE="${build_gomodcache}" GOCACHE="${build_gocache}" GOMAXPROCS=1 go build -p 1 -o x-ui main.go
    if [[ $? -ne 0 ]]; then
        echo -e "${red}源码构建失败，请检查 Go 环境与依赖下载是否正常${plain}"
        cleanup_source_build
        exit 1
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        rm /usr/local/x-ui/ -rf
    fi

    mkdir -p /usr/local/x-ui /etc/x-ui
    cp -r x-ui x-ui.sh bin web config database logger util v2ui xray /usr/local/x-ui/
    cp -f x-ui.service /etc/systemd/system/
    cp -f x-ui.sh /usr/bin/x-ui
    chmod +x /usr/local/x-ui/x-ui /usr/local/x-ui/x-ui.sh /usr/local/x-ui/bin/xray-linux-* /usr/bin/x-ui
    setup_xray_user
    chown -R :xray /usr/local/x-ui/bin
    chmod -R g+rX /usr/local/x-ui/bin
    last_version="source-main"
    cleanup_source_build
}

#Create dedicated xray user for privilege separation
setup_xray_user() {
    if ! id -u xray &>/dev/null; then
        useradd -r -s /sbin/nologin xray
        echo -e "${green}xray system user created${plain}"
    fi
}

#This function will be called when user installed x-ui out of sercurity
config_after_install() {
    echo -e "${yellow}出于安全考虑，安装/更新完成后需要强制修改端口与账户密码${plain}"
    read -p "确认是否继续?[y/n]": config_confirm
    if [[ x"${config_confirm}" == x"y" || x"${config_confirm}" == x"Y" ]]; then
        read -p "请设置您的账户名:" config_account
        echo -e "${yellow}您的账户名将设定为:${config_account}${plain}"
		read -s -p "请设置您的账户密码 (至少8位，需包含大写、小写、数字、符号中的3类):" config_password
		echo ""
        echo -e "${yellow}确认设定,设定中${plain}"
		result=$(/usr/local/x-ui/x-ui setting -username "${config_account}" -password "${config_password}" 2>&1)
		if echo "${result}" | grep -q "failed"; then
			echo -e "${red}账户密码设定失败: ${result}${plain}"
			echo -e "${red}密码要求: 至少8位，需包含大写字母、小写字母、数字、符号中的3类${plain}"
			echo -e "${red}请重新运行脚本或使用 x-ui 命令重置用户名密码${plain}"
		else
			echo -e "${yellow}账户密码设定完成${plain}"
		fi
        read -p "请设置面板访问端口:" config_port
        echo -e "${yellow}您的面板访问端口将设定为:${config_port}${plain}"
		/usr/local/x-ui/x-ui setting -port "${config_port}"
        echo -e "${yellow}面板端口设定完成${plain}"
    else
        echo -e "${red}已取消,所有设置项均为默认设置,请及时修改${plain}"
    fi
}

install_x-ui() {
    systemctl stop x-ui >/dev/null 2>&1 || true
    cd /usr/local/

    if [ $# == 0 ]; then
        last_version=$(curl -Ls "https://api.github.com/repos/motao123/x-ui/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        if [[ ! -n "$last_version" ]]; then
            install_from_source
            config_after_install
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
            show_install_success
            return 0
        fi
        echo -e "检测到 x-ui 最新版本：${last_version}，开始安装"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-${arch}.tar.gz https://github.com/motao123/x-ui/releases/download/${last_version}/x-ui-linux-${arch}.tar.gz
        if [[ $? -ne 0 ]]; then
            install_from_source
            config_after_install
            systemctl daemon-reload
            systemctl enable x-ui
            systemctl start x-ui
            show_install_success
            return 0
        fi
    else
        last_version=$1
        url="https://github.com/motao123/x-ui/releases/download/${last_version}/x-ui-linux-${arch}.tar.gz"
        echo -e "开始安装 x-ui v$1"
        wget -N --no-check-certificate -O /usr/local/x-ui-linux-${arch}.tar.gz ${url}
        if [[ $? -ne 0 ]]; then
            echo -e "${red}下载 x-ui v$1 失败，请确保此版本存在${plain}"
            exit 1
        fi
    fi

    if [[ -e /usr/local/x-ui/ ]]; then
        rm /usr/local/x-ui/ -rf
    fi

    tar zxvf x-ui-linux-${arch}.tar.gz
    rm x-ui-linux-${arch}.tar.gz -f
    cd x-ui
    chmod +x x-ui bin/xray-linux-${arch}
    setup_xray_user
    chown -R :xray bin
    chmod -R g+rX bin
    cp -f x-ui.service /etc/systemd/system/
    wget --no-check-certificate -O /usr/bin/x-ui https://raw.githubusercontent.com/motao123/x-ui/main/x-ui.sh
    chmod +x /usr/local/x-ui/x-ui.sh
    chmod +x /usr/bin/x-ui
    config_after_install
    #echo -e "如果是全新安装，默认网页端口为 ${green}54321${plain}，用户名和密码默认都是 ${green}admin${plain}"
    #echo -e "请自行确保此端口没有被其他程序占用，${yellow}并且确保 54321 端口已放行${plain}"
    #    echo -e "若想将 54321 修改为其它端口，输入 x-ui 命令进行修改，同样也要确保你修改的端口也是放行的"
    #echo -e ""
    #echo -e "如果是更新面板，则按你之前的方式访问面板"
    #echo -e ""
    systemctl daemon-reload
    systemctl enable x-ui
    systemctl start x-ui
    show_install_success
}

show_install_success() {
    echo -e "${green}x-ui v${last_version}${plain} 安装完成，面板已启动，"
    echo -e ""
    echo -e "x-ui 管理脚本使用方法: "
    echo -e "----------------------------------------------"
    echo -e "x-ui              - 显示管理菜单 (功能更多)"
    echo -e "x-ui start        - 启动 x-ui 面板"
    echo -e "x-ui stop         - 停止 x-ui 面板"
    echo -e "x-ui restart      - 重启 x-ui 面板"
    echo -e "x-ui status       - 查看 x-ui 状态"
    echo -e "x-ui enable       - 设置 x-ui 开机自启"
    echo -e "x-ui disable      - 取消 x-ui 开机自启"
    echo -e "x-ui log          - 查看 x-ui 日志"
    echo -e "x-ui v2-ui        - 迁移本机器的 v2-ui 账号数据至 x-ui"
    echo -e "x-ui update       - 更新 x-ui 面板"
    echo -e "x-ui install      - 安装 x-ui 面板"
    echo -e "x-ui uninstall    - 卸载 x-ui 面板"
    echo -e "----------------------------------------------"
}

echo -e "${green}开始安装${plain}"
install_base
install_x-ui $1
