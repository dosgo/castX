var inEndpoint,outEndpoint;
let readingActive = false; // 新增
let targetInterface = null; // 新增
var usbConnectd=false;
let device;
let wsUsb;
function connectDevice() {
    navigator.usb.requestDevice({  filters: [{ 
            classCode: 0xFF, 
            subclassCode: 0x42, 
            protocolCode: 0x01 
        }] })
    .then(selectedDevice => {
        device=selectedDevice;
        console.log(device.productName);      // "Arduino Micro"
        console.log(device.manufacturerName); // "Arduino LLC"
        return device.open(); 
    })
    .then(() => {
        return device.selectConfiguration(1)
    }) // Select configuration #1 for the device.
    .then(() => {
        for( iface of device.configuration.interfaces) {
            const alternate = iface.alternate; // 当前备选设置
            if (alternate.interfaceClass === 0xFF && 
                alternate.interfaceSubclass === 0x42 && 
                alternate.interfaceProtocol === 0x01) {
                    targetInterface = iface; // 保存接口引用
                return device.claimInterface(iface.interfaceNumber);
            }
        }
    }).then(()=> {
        for (const endpoint of targetInterface.alternate.endpoints) {
            if (endpoint.direction === 'in') {
                inEndpoint = endpoint;
            } else if (endpoint.direction === 'out') {
                outEndpoint = endpoint;
            }
        }
        usbConnectd=true;
        wsUsb = new WebSocket(`ws://${location.host}/usbWs`);
        wsUsb.binaryType = 'arraybuffer';
        wsUsb.onopen = () => {
           startUsbReadingLoop();
        };
        wsUsb.onclose = () => {
            console.log('WebSocket 连接已关闭');
            closeUsb();
        };
        wsUsb.onmessage = (event) => {
            var data=event.data;
            try {
                // 转换为二进制数据
                let binaryData;
                if (typeof data === 'string') {
                    // 文本消息
                    binaryData = new TextEncoder().encode(data);
                } else if (data instanceof ArrayBuffer) {
                    // 二进制消息
                    binaryData = new Uint8Array(data);
                } else if (data instanceof Blob) {
                    // Blob 消息
                    binaryData = new Uint8Array(data.arrayBuffer());
                } else {
                    throw new Error('不支持的 WebSocket 数据类型');
                }
                device.transferOut(outEndpoint.endpointNumber, binaryData);
            } catch (error) {
                log(`转发 WebSocket 消息错误: ${error.message}`, 'error');
                wsUsb.close();
            }
        };
    }).catch(error => { console.error(error); });
}

async  function startUsbReadingLoop() {
    readingActive = true;
    try {
        while (readingActive) {
            let  result = await device.transferIn(inEndpoint.endpointNumber, inEndpoint.packetSize*32);
            if (!readingActive) break; // 检查是否已停止
            if (result.status !== 'ok') {
                log(`USB 传输错误: ${result.status}`, 'error');
                return ;
            }
            if (result.data && result.data.byteLength > 0) {
                const data = result.data;
                try {
                    wsUsb.send(data);
                } catch (error) {
                    log(`WebSocket 发送错误: ${error.message}`, 'error');
                }
            }
        }
    } catch (error) {
        log(`USB 读取错误: ${error}`);
    }
}
window.addEventListener('beforeunload', closeUsb);
function closeUsb(){
    readingActive = false; // 停止读取
    if (device) {
        //send err data close
        device.transferOut(outEndpoint.endpointNumber,  new Uint8Array(2));
        device.forget().catch(() => {}); 
    }
}


