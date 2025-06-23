<?php

$uas = [
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.5735.199 Safari/537.36',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/113.0',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36 Edg/114.0.1823.51',
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 13_4_1) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Safari/605.1.15',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36 OPR/90.0.4480.84',
    'Mozilla/5.0 (Linux; Android 13; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.5735.199 Mobile Safari/537.36',
    'Mozilla/5.0 (iPhone; CPU iPhone OS 16_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1',
    'Mozilla/5.0 (Android 13; Mobile; rv:109.0) Gecko/113.0 Firefox/113.0',
    'Mozilla/5.0 (iPad; CPU OS 16_5_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.5 Mobile/15E148 Safari/604.1',
    'Mozilla/5.0 (Linux; Android 13; SM-T970) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.5735.199 Safari/537.36',
    'Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)',
    'Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)',
    'Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)',
    'Mozilla/5.0 (Nintendo Switch; WebApplet) AppleWebKit/601.6 (KHTML, like Gecko) NF/4.0.0.5.9 NintendoBrowser/5.0.0.12.46',
    'Mozilla/5.0 (PlayStation 5 1.00) AppleWebKit/537.36 (KHTML, like Gecko) PS5Browser/1.00',
];

$referrers = [
    'https://www.google.com/',
    'https://www.bing.com/',
    'https://www.baidu.com/',
    'https://www.yahoo.com/',
    'https://www.duckduckgo.com/',
    'https://www.ask.com/',
    'https://www.yandex.com/',
    'https://www.baidu.com/',
    'https://so.com',
    'https://potawang.cn',
];
$paths     = [
    'soft',
    'news',
    'zt',
    'info',
    '',
    ''
];
$cidrs = require 'cidr.php';

function generateRandomIPv4($cidrs) {
    $cidr = $cidrs[array_rand($cidrs)];
    return generateRandomIP($cidr);
}

function generateRandomIP($subnet) {
    // 分割网络地址和子网掩码长度
    list($network, $netmask) = explode('/', $subnet);
    $netmask = intval($netmask);

    // 将网络地址转换为整数
    $networkLong = ip2long($network);

    // 计算子网掩码的整数表示
    $maskLong = -1 << (32 - $netmask);

    // 计算起始 IP 和结束 IP
    $startIP = $networkLong & $maskLong;
    $endIP = $startIP + pow(2, (32 - $netmask)) - 1;

    // 随机生成一个范围内的 IP 地址
    $randomIP = mt_rand($startIP, $endIP);

    // 将整数转换回 IP 地址
    return long2ip($randomIP);
}

for ($i = 0; $i < 200; $i++) {
    $curl     = curl_init();
    $ua       = $uas[array_rand($uas)];
    $path     = $paths[array_rand($paths)];
    $referrer = $referrers[array_rand($referrers)];
    $ip = generateRandomIPv4($cidrs);
    // $ip = '1.0.1.133';
    // $ua = 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/89.0.4389.90 Safari/537.36';
    // $referrer = 'https://www.google.com/';

    curl_setopt_array($curl, array(
        CURLOPT_URL => 'http://127.0.0.1:8080/api/event',
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_ENCODING => '',
        CURLOPT_MAXREDIRS => 10,
        CURLOPT_TIMEOUT => 0,
        CURLOPT_FOLLOWLOCATION => true,
        CURLOPT_HTTP_VERSION => CURL_HTTP_VERSION_1_1,
        CURLOPT_CUSTOMREQUEST => 'POST',
        CURLOPT_POSTFIELDS => sprintf('{
    "d": "potawang.cn",
    "n": "pageview",
    "p": {%s},
    "r": "%s",
    "u": "https://potawang.cn/%s",
    "v": "1.0",
    "h": %s
}', '"source":"so.com", "classid":"detail"', $referrer, $path, rand(0, 1)),
        CURLOPT_HTTPHEADER => array(
            'User-Agent: ' . $ua,
            'Content-Type: text/plain',
            'Accept: */*',
            'Host: 127.0.0.1:8080',
            'Connection: keep-alive',
            'X-Forwarded-For: ' . $ip,
        ),
    ));

    $response = curl_exec($curl);

    curl_close($curl);
    // 打印http code
    // $code = curl_getinfo($curl, CURLINFO_HTTP_CODE);
    // echo "code: " . $code . "\n";

}
// Hash           int8           `form:"h" json:"h"`   // 哈希
// EventName      string         `form:"n" json:"n"`   // 事件名称
// JSVersion      string         `form:"v" json:"v"`   // JS版本
// URL            string         `form:"u" json:"u"`   // URL
// Domain         string         `form:"d" json:"d"`   // 域名
// Referrer       string         `form:"r" json:"r"`   // 来源
// Props          map[string]any `form:"p" json:"p"`   // 属性
// EngagementTime int            `form:"e" json:"e"`   // 参与时间
// ScrollDepth    uint8          `form:"sd" json:"sd"` // 滚动深度