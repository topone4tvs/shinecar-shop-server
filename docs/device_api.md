#### 1、协议概述

###### 1.1 HTTP服务器配置

1. 使用 HTTP 推送首先需要用户建立一个 HTTP 服务器，同时将这台 HTTP 服务器的地址配置给相机一体机；

机；

2. 当一体机有识别结果后（或者其他需要推送的内容时），就会往指定的服务器地址发送 HTTP 协议消息；

3. 在一体机网页，登录后，点击菜单栏->高级设置->HTTP 推送，进入到 HTTP 推送的设置界面(不同版本稍有区别)；

4. 接收 HTTP 推送的服务器，配置包括地址（可以填 ip 地址或者域名），端口号，是否开启 ssl 连接，ssl端口号，和超时时间设置。请根据架设的服务器的情况进行配置；

5. HTTP 服务器可配置 1 个主服务器，最多 3 个备选服务器；若勾选了主服务器优先，则相机只往主服务器推送，当且仅当主服务器断开连接时，才会往备选服务器推送；若没勾选主服务优先，则会同时往主/备服务器推送数据；

6. HTTP 推送具体配置，即 HTTP 需要推送的内容，包括车牌识别结果、IO 触发、串口 485 数据，需要配置相应推送的 url；

7. HTTP 心跳分为取消心跳、普通心跳、comet 轮询；普通心跳相机定时往主服务器推送心跳，不处理主服务器的业务回复；comet 轮询则一直与服务器推送心跳交互，并且服务器可在回复消息中携带业务处理；

8. HTTP 脱机检查，当开启脱机检查时，相机会对 HTTP 进行脱机检查；脱机检查分为心跳检查以及识别结果检查；心跳检查则为普通心跳定时检查；识别结果检查为当产生识别结果时，推送识别结果后需要在检查时间内收到服务器的回复，否则相机会置状态为脱机；当相机处于脱机时，会进行脱机相关的业务处理；

9. 配置重发次数，最多可配置 4 次，即当产生推送时，若没推送成功，相机会对推送数据进行重发；

#### 2、协议定义

###### 2.1 车牌识别结果推送

###### 2.1.1 识别结果结构定义

开启推送车牌识别结果,同时配置推送 url 后，当有车牌识别结果产生时，相机会按图中的配置会发送消息到: http://192.168.1.106/devicemanagement/php/plateresult.php；

具体结果数据结构如下：

```json
{
    "AlarmInfoPlate":
    {
        "channel": 0, // 默认通道号（预留）
        "deviceName": "R5-V1.0", // 设备名称
        "ipaddr": "192.168.55.131", // 设备ip地址
        "result": // 实际数据
        {
            "PlateResult": // 车牌识别结果信息
            {
                "bright": 0, // 亮度评价
                "carBright": 0, // 车身亮度
                "carColor": 255, // 车身颜色
                "car_brand": // 车辆品牌信息
                {
                    "brand": 255, // 品牌ID，取值参见附录
                    "type": 255, // 车辆类型， 取值参见附录
                    "year": 65535, // 车系编码
                },
                "carlocation": // 车头位置信息
                {
                    "RECT": // 位置举行区域
                    {
                        "bottom": 0,
                        "left": 0,
                        "right": 0,
                        "top": 0
                    }
                },
                "clean_time": 0, // 未使用
                "colorType": 4, // 车牌颜色，取值参见附录
                "colorValue": 0, // 车牌颜色
                "confidence": 96, // 识别结果可信度，1-100
                "direction": 0, // 车的行进方向，取值参见附录
                "feature_code": true, // 车辆特征码
                "imageFile": "", // 识别结果的大图内容经过base64编码后的字符串，需要在后台开启发送大图片
                "imageFileLen": 0, // 识别结果的图片大小
                "imageFragmentFile": "", // 识别结果的小图片内容经过base64编码后的字符串，需要在后台开启发送小图片
                "imageFragmentFileLen": 0, // 识别结果的图片大小
                "gioouts": // 开闸输出信息
                [
                    {
                        "ctrltype": 0, // 开闸类型，取值参见附录
                        "ionum": "0" // IO OUT序号，当前最大4个IOout
                    },
                    {
                        "ctrltype": 0,
                        "ionum": "1"
                    },
                    {
                        "ctrltype": 0,
                        "ionum": "2"
                    }
                ],
                "is_fake_plate": 0, // 是否伪车牌，0：真实车牌，1：伪车牌
                "isoffline": 0, // 设备离线状态，0：在线 1：离线
                "license": "粤Z9065澳", // 车牌号字符串
                "license_ext_type": 0, // 新式小型车牌扩展，仅在车牌为小型车牌的时候有效
                "location": // 车牌在图片中的位置
                {
                    "RECT": // 位置矩形区域
                    {
                        "bottom": 674,
                        "left": 1002,
                        "right": 1263,
                        "top": 565
                    }
                },
                "plate_distance": 0, // 车牌距离
                "plate_true_width": 25, // 车牌真实宽度
                "plateid": 21085, // 识别结果车牌ID
                "plates": // 车牌列表
                [
                    {
                        "color": 4, // 颜色
                        "license": "粤Z9065澳", // 车牌号字符串
                        "plate_width": 281, // 车牌宽度
                        "pos": // 车牌坐标
                        {
                            "bottom": 674,
                            "left": 1002,
                            "right": 1263,
                            "top": 565
                        },
                        "type": 14 // 车辆类型
                    }
                ],
                "timeStamp": // 识别结果对应帧的时间戳
                {
                    "Timeval":
                    {
                        "decday": 9, // 天
                        "dechour": 16, // 小时
                        "decmin": 39, // 分钟
                        "decmon": 6, // 月
                        "decsec": 54, // 秒
                        "decyear": 2023, // 年
                        "sec": 1686299994, // 标准时间戳
                        "usec": 687772 // 毫秒时间戳
                    }
                },
                "timeUsed": 0, // 识别所用时间
                "triggerType": 4, // 当前结果的触发类型，取值参见附录
                "type": 14 // 车牌类型，取值参见附录
            }
        },
        "rule_id": 1, // 规则ID标识
        "serialno": "fe71e106-57263272", // 设备序列号
        "user_data": "" // 用户自定义数据
    }
}
```

###### 2.1.2 断线重传

1. 当 HTTP 服务器因为某些原因，导致相机与服务器断线以后，相机会把推送失败的识别结果记为离线记录，当服务器重新连接上以后，相机根据配置判断是否需要推送离线记录，同
   时发送离线记录；
2. 配置在网页配置，开启断线重传功能，注意当取消断线重传功能时，会清空当前相机的离线记录；
3. 相机推送识别结果，相对于旧版本的推送消息，新增三个字段：plateid, isoffline, gioouts, 脱机记录 isoffline 的值为 1；
4. 服务器回复相机识别结果时，在线记录需要将 plateid 字段值回复到相机消息中；
5. 服务器回复离线识别结果时，需要回复是否继续接收离线记录以及接收到最新的 plateid；
6. 离线脱机记录理论上最大支持 9000 条离线记录的重新推送；
7. 在推送离线记录的过程中，如果发生新的识别结果，优先推送新的识别结果，此时未完成推送的离线记录，会直接终止处理，当新识别结果推送完成之后，方重新开始推送离线记
   录；
   注意：当推送离线记录，相机还未收到服务器的回复时，产生了新的识别结果，相机会终止上一条离线记录的推送处理，直接推送新的识别结果，当新的识别结果推送完毕后，再推送
   上一条离线记录，故服务器此时有可能会收到两条一模一样的离线记录，服务器可根据 plateid 进行过滤；

相机推送的数据

```json
{
    "AlarmInfoPlate":
    {
        "channel": 0,
        "deviceName": "IVS",
        "ipaddr": "192.168.1.100",
        "result":
        {
            "PlateResult":
            {
                "bright": 0,
                "carBright": 0,
                "carColor": 0,
                "colorType": 0,
                "confidence": 0,
                "direction": 0,
                "imagePath": "xxxxx.jpg",
                "license": "_无_",
                "location":
                {
                    "RECT":
                    {
                        "bottom": 0,
                        "left": 0,
                        "right": 0,
                        "top": 0
                    }
                },
                "timeStamp":
                {
                    "Timeval":
                    {
                        "decday": 8,
                        "dechour": 10,
                        "decmin": 26,
                        "decmon": 6,
                        "decsec": 28,
                        "decyear": 2018,
                        "sec": 1441815171,
                        "usec": 672241
                    }
                },
                "timeUsed": 0,
                "triggerType": 4,
                "type": 0,
                "plateid": 123,
                "isoffline": 0, // 设备离线状态，0：在线，1：离线
                "gioouts":
                [
                    {
                        "ionum": 1, // IO OUT 序号，当前最大 4 个 IOout
                        "ctrltype": 0 // 取值范围[0, 2] 开闸类型：HTTP_IO_OUT_STATUS
                    }
                ]
            }
        },
        "serialno": "eff50e18-e3d3862b"
    }
}
```

服务器回复：

```json
{
    "Response_AlarmInfoPlate":
    {
        "ContinuePushOffline":
        {
            "plateid": 123, // 推送的离线车牌记录 ID
            "continue": 1 // 是否继续推送离线记录，0：否，1：是
        }
    }
}
```

> 注意：只有当服务器回复了离线记录消息，以及 continue 字段为 1,时，才继续推送下一条离线记录；

###### 2.2 端口触发信息推送

当开启时，如果在输入输出页面->车牌触发方式里，开启了外部输入 1 触发或者 2 触发，输入有变化时，会推送 json 格式数据，内容如下：

```json
{
    "AlarmGioIn":
    {
        "deviceName": "IVS",
        "ipaddr": "192.168.109.40",
        "result":
        {
            "TriggerResult":
            {
                "source": 3,
                "value": 1 // 表示触发时输入的状态
            }
        },
        "serialno": "d85b1269-8f942256"
    }
}
```

其中，TriggerResult 中：
source=0 代表是 IO 输入 1；
source=1 代表是 IO 输入 2；
source=2 代表是 IO 输入 3；
source=3 代表是 IO 输入 4；
source=4 代表输入 TCP 触发输入；

###### 2.3 串口数据推送

当开启串口数据推送时，配置了 url，在相机收到 485 数据时，会主动往服务器地址推送485数据；

```json
{
    "SerialData":{
        "channel" : 0, //通道号，当前为 0
        "serialno" : "cead13eb-1a198cd7", //设备序列号
        "ipaddr" : "192.168.1.100", //设备 ip
        "deviceName" : "IVS", //设备名称
        "serialChannel" : 0, //串口的通道号，通道 0 为 485 口 1，通道 1 根据 跳线方式为 485 口 2 或者 232
        "data": "Y2guY29tFw==", //串口数据，采用 base64 编码
        "dataLen" : 7 //串口数据实际长度
    }
}
```

###### 2.4 截图数据

用户在 comet 轮询或者收到识别结果的回复字段有获取截图时，设备会进行当前视频截图并上传，imageFIle 字段为图片 base64 后的编码，imageFileLen 为编码前的图片长度

```json
{
    "ipaddr" : "192.168.1.100",
    "TriggerImage":
    {
        "imageFile":"Y2guY29tFw==", //图片数据（base64 编码）
        "imageFileLen":7 //图片数据实际长度
    }
}
```

###### 2.5 设备注册

###### 2.5.1 普通心跳

1. 当相机网页配置设备注册状态为普通心跳时，相机会定时往主服务器推送心跳消息：
2. 当主服务连接正常，开启脱机检查的情况下，相机每隔 5S 左右推送一次心跳；
3. 当主服务连接正常，同时没开启脱机检查的情况下，是 30S 推送一次心跳消息；
4. 当主服务心跳丢失以后，相机每隔 1S 尝试连接一次；
5. 心跳推送使用 HTTP 长连接，POST 方式；

数据内容，使用 mutipart/form-data text 类型的数据格式：

```text
192.168.109.40--------------------------caa771fe4a61f3d9Content-Disposition: form-data;
name="device_name"IVS--------------------------caa771fe4a61f3d9Content-Disposition: form-data;
name="ipaddr"192.168.109.40--------------------------caa771fe4a61f3d9Content-Disposition:
form-data;
name="port"80--------------------------caa771fe4a61f3d9Content-Disposition: form-data;
name="user_name"admin--------------------------caa771fe4a61f3d9Content-Disposition:
form-data;
name="pass_wd"admin--------------------------caa771fe4a61f3d9Content-Disposition: form-data;
name="serialno"d85b1269-8f942256--------------------------caa771fe4a61f3d9Content-Disposition:
form-data;
name="channel_num"1--------------------------caa771fe4a61f3d9--
```

###### 2.5.2 comet轮训

1. 当开启 comet 轮询之后，相机会一直与 HTTP 服务器进行交互，保持连接请求，相机主动发送设备注册消息，内容与普通心跳内容一致，收到回复时，立即发送下一条消息；
2. 发送设备注册消息，与普通心跳消息保持一致；
3. comet 轮询会根据服务器回复做相应处理；

###### 2.6 业务处理

1. 相机根据服务器的回复消息，进行相应的业务处理；
2. 当前仅支持车牌识别结果的推送回复，以及 comet 轮询的消息回复，相机会根据回复做业务处理；
3. 可以多个消息组合发送例如系统播报语音同时通知 IO[0] 命令如下：

```json
{
    "Response_AlarmInfoPlate":
    {
        "playserver_json_request":
        {
            "type": "ps_voice_play",
            "voice": "5qyi6L+O5YWJ5Li0",
            "voice_interval": 0,
            "voice_volume": 100,
            "voice_male": 1
        },
        "ivs_ioctrl":
        {
            "delay": 500,
            "io": 0,
            "value": 2
        }
    }
}
```

###### 2.6.1 控制IO开闸

服务器在收到车牌识别结果推送、或者 comet 轮询时，回复以下结构的消息，可触发开闸

```json
{
    "Response_AlarmInfoPlate":
    {
        "info":"ok",//回复 ok 开闸
        // ....其他数据
    }
}
```

###### 2.6.2 控制串口推送 485 数据

服务器在收到车牌识别结果推送、或者 comet 轮询时，回复以下结构的消息，可发送485数据

```json
{
    "Response_AlarmInfoPlate":
    {
        "serialData" :
        [
            {
                "serialChannel":0,
                "data" : "...",
                "dataLen" : 123
            },
            {
                //数据 1，可以有或者没有，收到后将发送到对应串口
                "serialChannel":1,
                "data" : "....",
                "dataLen" : 123
            }
            //数据 2，可以有或者没有，收到后将发送到对应串口
        ]
        // ....其他数据
    }
}
```

###### 2.6.3 截图

服务器在收到车牌识别结果推送、或者 comet 轮询时，回复以下结构的消息，可触发截图：
相机会触发当前的视频截图，然后将截图数据推送到 snapImageAbsolutelyUrl 字段指定的服务器地址；

```json
{
    "Response_AlarmInfoPlate":
    { 
        "TriggerImage" :
        {
            //回复截图内容端口号（可选，不填则默认使用 http 页面配置端口）
            "port":80, //回复截图内容相对路径（可选，不触发截图可不添加该字段）
            "snapImageRelativeUrl" : "/devicemanagement/php/receivedeviceinfo.php", //回复截图内容绝对路径（可选，不触发截图可不添加该字段）
            "snapImageAbsolutelyUrl" :"http://192.168.1.106/devicemanagement/php/receivedeviceinfo.php"
        }
        // ....其他数据
    }
}
```

###### 2.6.4 手动触发识别

服务器在收到车牌识别结果推送、或者 comet 轮询时，回复以下结构的消息，可触发手动识别：

```json
{
    "Response_AlarmInfoPlate":
    {
        "manualTrigger" : "ok",//回复 ok 进行手动触发
        // ....其他数据
    }
}
```

或者仅回复以下数据：

```json
{
    "type":"AVS_TRIGGER"
}
```

此时会触发相机的手动识别，设备会将识别结果数据推送至 http 服务端，前提是服务端配置了识别数据的推送；

###### 2.6.5 系统预置语音播报

服务器在收到车牌识别结果推送、或者 comet 轮询时，回复以下结构的消息

```json
{
    "Response_AlarmInfoPlate":
    {
        "playserver_json_request":
        {
            "type":"ps_voice_play", // ps_voice_play 语音播放
            "voice":"JXU2QjIyJXU4RkNFJXU1MTQ5JXU0RTM0", // 语音信息，utf-8/GBK 编码的 BASE64 编码字符串
            "voice_interval":0, // 语音文件播放间隔
            "voice_volume":100, // 语音文件音量大小[1-100]
            "voice_male":1 // 语音类型：0 男声；1 女声
        }
    }
}
```

###### 2.6.6 IO控制

服务器在收到车牌识别结果推送、或者 comet 轮询时，回复以下结构的消息

```json
{
    "Response_AlarmInfoPlate":
    {
        "ivs_ioctrl":
        {
            "delay": 500, // 先通后断的延迟时间(uint32，单位:ms)；当 value 为 2 的时候有效, 最大 600*1000
            "io": 0, // ioout 的 id[0-3] 不同产品 io 个数不一样
            "value": 2 // 输出 IO 的状态值: 0 断, 1 通, 2 先通后断
        }
    }
}
```

#### 3、系统码表

###### 3.1 触发类型数据表

| 触发类型          | 编码  |
| ------------- | --- |
| 自动触发类型        | 1   |
| 外部 输入触发（IO输入） | 2   |
| 软件触发（SDK）     | 4   |
| 虚拟线圈触发        | 8   |
| 车滞留事件         | 64  |
| 车滞留恢复事件       | 65  |
| 车折返事件         | 66  |



###### 3.2 车牌类型数据表

| 车牌类型           | 编码  |
| -------------- | --- |
| 未知车牌           | 0   |
| 蓝牌小汽车          | 1   |
| 黑牌小汽车          | 2   |
| 单排黄牌           | 3   |
| 双排黄牌           | 4   |
| 警车车牌           | 5   |
| 武警车牌           | 6   |
| 个性化车牌          | 7   |
| 单排军车牌          | 8   |
| 双排军车牌          | 9   |
| 使馆车牌           | 10  |
| 香港进出中国大陆车牌     | 11  |
| 农用车牌           | 12  |
| 教练车牌           | 13  |
| 澳门进出中国大陆车牌     | 14  |
| 双层武警车牌         | 15  |
| 武警总队车牌         | 16  |
| 双层武警总队车牌       | 17  |
| 民航车牌           | 18  |
| 新能源车牌          | 19  |
| 大型新能源          | 20  |
| 应急车            | 21  |
| 领事馆            | 22  |
| 新标准燃油车         | 23  |
| 新标准新能源         | 24  |
| 机场车牌           | 25  |
| 海外车牌(所有海外车牌类型) | 26  |
| 假车牌            | 29  |
| 车标             | 30  |
| 无牌车            | 31  |



###### 3.3 车牌颜色数据表

| 车牌颜色 | 编码  |
| ---- | --- |
| 未知   | 0   |
| 蓝色   | 1   |
| 黄色   | 2   |
| 白色   | 3   |
| 黑色   | 4   |
| 绿色   | 5   |
| 黄绿色  | 6   |



###### 3.4 车身颜色数据表

| 车身颜色 | 编码  |
| ---- | --- |
| 白    | 0   |
| 银（灰） | 1   |
| 黄    | 2   |
| 粉    | 3   |
| 红    | 4   |
| 绿    | 5   |
| 蓝    | 6   |
| 棕    | 7   |
| 黑    | 8   |
| 未知   | 255 |



##### 3.5 运动方向数据表

| 运动方向 | 编码  |
| ---- | --- |
| 未知   | 0   |
| 左    | 1   |
| 右    | 2   |
| 上    | 3   |
| 下    | 4   |



###### 3.6 车辆类型数据表

| 车辆类型 | 编码   |
| ---- | ---- |
| 未知   | 0X00 |
| 轿车   | 0X01 |
| SUV  | 0X02 |
| MPV  | 0X03 |
| 小型客车 | 0X04 |
| 大型客车 | 0X05 |
| 小型货车 | 0X06 |
| 大型货车 | 0X07 |



###### 3.7 开闸类型

| 开闸类型      | 编码  |
| --------- | --- |
| 无开闸       | 0   |
| 服务器触发开闸   | 1   |
| 脱机状态白名单开闸 | 2   |



#### 4、常见问题

###### 4.1 Q：设备注册是什么？
A：当开启时，每隔一段时间，一体机会自动发送设备信息到中心服务器，包括设备 ip，端口，序列号等信息；

###### 4.2 Q：设置好了，请求收不到，什么问题？
A：请确保一体机可以访问中心服务器的相应地址。常见的问题如，局域网内，网线是否接好，ip 地址是否冲突，是否在可以访问的网段；中心服务器如果在公网，请确保一体机可以访问公网，需要设置好一体机的网关和 dns 地址。检查中心服务器是否运行；

###### 4.3 Q：请求收到了，但没有数据（数据格式不对）？
A：车牌识别结果推送的请求发送的是 json 数据，http 的 body 内容如；
```json
{"AlarmInfoPlate":{…}}
```

###### 4.4 Q：设备注册又是什么格式？
A：设备注册请求发送的数据内容如下：
```text
------------------------------cd9a1a32759bContent-Disposition: form-data; name="device_name"IVS------------------------
------cd9a1a32759bContent-Disposition: form-data; name="ipaddr"192.168.0.100------------------------------cd9a1a32
759bContent-Disposition: form-data; name="port"80------------------------------cd9a1a32759bContent-Disposition: fo
rm-data; name="user_name"admin------------------------------cd9a1a32759bContent-Disposition: form-data; name=" pass_wd"admin------------------------------cd9a1a32759bContent-Disposition: form-data; name="serialno"fcb68a83-e
e8409dd------------------------------cd9a1a32759bContent-Disposition: form-data; name="channel_num"1---------------
---------------cd9a1a32759b--
```
如所见是 formpost 的格式，接收方法例如：java 使用 request.getQueryString 接收，php 使用$_POST 变量接收；

###### 4.5 Q：如何回复请求开闸？
A：回复
```json
{"Response_AlarmInfoPlate":{"info":"ok","content":"...","is_pay":"true"}}
```
info 如果是 ok 表示开闸；

###### 4.6 Q：回复中 content 能不能是中文？
A：所有请求都用 utf8 进行编码，回复也用 utf8 即可；

###### 4.7 Q：能否使用 ssl 连接发送，我们的中心服务器是 ssl 的？
A：在设置中设置 ssl 端口（一般是 443），然后选上开启，设置就可以了，注意如果中心服务器不支持 ssl 连接，请不要选择开启该项；

###### 4.8 Q：怎么获取截图？
A：推送的结果中有"imagePath": "/snapshot/lpr/tri_snap_24.jpg"，后面是访问截图的 http 路径，前面加上一体机的网址，就可以得到截图的地址如 http://192.168.1.100:8080/snapshot/lpr/tri_snap_24.jpg；

###### 4.9 Q：为什么相同车牌返回了两次结果？
A: 推送的结果中有一项触发类型 triggerType，可以根据触发类型来过滤结果；
