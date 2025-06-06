let pc = null;
let remoteVideo = document.getElementById('remoteVideo');
let GOOS='';
let nativeWidth = 0; // 手机原始宽度
let nativeHeight = 0; // 手机原始高度
let videoHeight=0;//视频原始高度
let videoWidth=0;//视频原始宽度
let targetWidth=0;
let targetHeight=0;
let orientation=0;//默认方向
var securityKey=""
let log = msg => {
    document.getElementById('logs').innerHTML += msg + '<br>'
}



const ws = new WebSocket(`ws://${location.host}/ws`);
ws.onopen = () => {
    log('websocket connected');
};
ws.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    if (msg.type === 'offerResponse') {
        pc.setRemoteDescription(msg.data.sdp);
    }
    if (msg.type === 'infoNotify') {
        orientation = msg.data.orientation;
        nativeWidth  = msg.data.width;
        nativeHeight  = msg.data.height;
        videoHeight = msg.data.videoHeight;
        videoWidth  = msg.data.videoWidth;
        
        //如果使用了adb
        if (msg.data.useAdb==true) {
            if (typeof appvm !== 'undefined'){
                appvm.isConnected=msg.data.adbConnect;
            }
            if (typeof videoVm !== 'undefined'){
                videoVm.isAndroid=true;
                videoVm.useAdb=true;

            }
        }
        //如果是canvas渲染
        if (typeof canvasSizev1 === 'function') {
            canvasSizev1(); // 
        }
    }
    //初始化配置
    if (msg.type === 'initConfig') {
        securityKey  = msg.data.securityKey;
        GOOS=msg.data.GOOS;
        if(GOOS=='android'){
            document.querySelectorAll('.androidMenu').forEach(el => {
                el.style.display = 'inline-block'; // 或 flex/grid/inline-block 等
            });
            if (typeof videoVm !== 'undefined'){
                videoVm.isAndroid=true;
            }
        }
        login();//请求登录
    }
    //登录成功
    if (msg.type === 'loginAuthResp') {
        if(msg.data.auth){
            if (typeof videoVm !== 'undefined'){
                videoVm.isAuth=true;
                videoVm.errorMessage="";
            }
            initWebRTC();
        }else{
            if (typeof videoVm !== 'undefined'){
                videoVm.errorMessage=getLang('loginErrMsg');
            }
        }
    }
};

function login() {
    let authInfo= getToken()
    let args={"maxSize":screen.width>screen.height?screen.width:screen.height}
    args.timestamp=authInfo['timestamp'];
    args.token=authInfo['token'];
    ws.send(JSON.stringify({
        type: 'loginAuth',
        data: JSON.stringify(args)
    }));
}
function initWebRTC() {
    pc = new RTCPeerConnection()
    pc.addTransceiver('video')
    pc.addTransceiver('audio')

    pc.oniceconnectionstatechange = () => log(pc.iceConnectionState)
    pc.ontrack = function (event) {
        if (event.track.kind === 'video') {
                console.log('收到视频轨道');
                remoteVideo.autoplay = true
                remoteVideo.muted = false; 
                remoteVideo.srcObject = event.streams[0];
                console.log( 'streams',event.streams[0]);
        }
    }
    sendOffer(false);
}

// 发送SDP Offer
async function sendOffer(iceRestart) {
    const offer = await pc.createOffer({iceRestart});
    await pc.setLocalDescription(offer);
    ws.send(JSON.stringify({
        type: 'offer',
        data: JSON.stringify(offer)
    }));
}
function keyboardClick(code) {
    var args= JSON.stringify({"type":'keyboard',"code":code,"videoWidth":remoteVideo.videoWidth,"videoHeight":remoteVideo.videoHeight})
    ws.send(JSON.stringify({
        type: 'control',
        data: args
    }));
}


function swipe(code) {
    var args=  JSON.stringify({"type":'swipe',"code":code,"videoWidth":remoteVideo.videoWidth,"videoHeight":remoteVideo.videoHeight})
    ws.send(JSON.stringify({
        type: 'control',
        data: args
    }));
}


function mouseClick(type,x,y,touch) {
    var args=  JSON.stringify({"type":type,"x":x,"y":y,"offsetWidth":remoteVideo.offsetWidth,"offsetHeight":remoteVideo.offsetHeight,'touch':touch})   
    ws.send(JSON.stringify({
        type: 'control',
        data: args
    }));
}



function checkDevice() {
    const ua = navigator.userAgent;
    const isTouch = 'ontouchstart' in window || navigator.maxTouchPoints > 0;

    // 移动设备特征
    const isMobileUA = /(iPhone|iPod|Android|BlackBerry|Windows Phone)/i.test(ua);
    
    // 平板特征（如 iPad Pro）
    const isTablet = /iPad/i.test(ua) || (isTouch && !/Mobi/i.test(ua) && window.screen.width >= 768);

    return isMobileUA || isTablet ? "mobile" : "desktop";
}

 function getToken(){
   let  password=localStorage.getItem('password')||'';
   const timestamp = Date.now();
   let src=securityKey+"|"+timestamp+"|"+password;
    let digestBuffer=sha256(src);
    return {
        token: digestBuffer,
        timestamp,
    };
}


