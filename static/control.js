
var videoObj = document.getElementById('remoteVideo');

// 指针按下（兼容鼠标、触摸）
let isPointerDown = false;
var startX=0;
var startY=0;
var startTime=0;
var touchNum=10;
var lastX=0;
var lastY=0;
videoObj.addEventListener('pointerdown', (e) => {
  e.preventDefault();
  panstart(e);
});

videoObj.addEventListener('touchstart', (e) => {
  e.preventDefault();
  panstart(e);
});


function panstart(e){
  isPointerDown = true;
  const touch = e.touches ? e.touches[0] : e;
  let clientX = touch.offsetX?touch.offsetX:touch.clientX;
  let clientY = touch.offsetY?touch.offsetY:touch.clientY;
  startX=clientX;
  startY=clientY;
  var pos= fixXy(clientX,clientY);
  console.log(`pos`, pos);
  startTime=Date.now();
  mouseClick('panstart', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 0);
}
// 指针移动
videoObj.addEventListener('pointermove', (e) => {
  e.preventDefault();
  if(!isPointerDown){
      return;
  }
  const touch = e.touches ? e.touches[0] : e;
  let clientX = touch.offsetX?touch.offsetX:touch.clientX;
  let clientY = touch.offsetY?touch.offsetY:touch.clientY;
  if(clientX<0){
    clientX=0;
  }
  if(clientY<0){
    clientY=0;
  }
  lastX=clientX;
  lastY=clientY;
  var pos= fixXy(clientX,clientY);
  console.log(`pos`, pos);
  mouseClick('pan', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 0);

});

videoObj.addEventListener('touchend', (e) => {
  console.log('touchend ');
  console.log(e);
  if (isPointerDown){
    clickUp(e,false);
  }
});
// 指针释放
videoObj.addEventListener('pointerup', (e) => {
  clickUp(e,false);
});

function clickUp(e,outside) {
    e.preventDefault();
    let touch = e.touches ? e.touches[0] : e;
    if(touch==null){
       touch=e.changedTouches[0];
    }
    let clientX = touch.offsetX?touch.offsetX:touch.clientX;
    let clientY = touch.offsetY?touch.offsetY:touch.clientY;
    if(clientX<0){
      clientX=0;
    }
    if(clientY<0){
      clientY=0;
    }
    if(outside){
      clientX=lastX;
      clientY=lastY;
    }
    console.log(`clientX`, clientX);
    console.log(`clientY`, clientY);
    var pos= fixXy(clientX,clientY);
    console.log(`clickUp pos`, pos);
  
    if(Math.abs(clientX-startX)  < touchNum&&  Math.abs(clientY  -startY ) < touchNum || !isPointerDown ){
      startX=0;
      startY=0; 
      let duration = Date.now() - startTime;
      if(duration<20){
        duration=20;
      }
      startTime=0;
      mouseClick('click', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, duration);
      isPointerDown = false;
      return; // 忽略点击事件，防止误触
    }
    isPointerDown = false;
    mouseClick('panend', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 0);
    startTime=0;
}

/** */
function fixXy( relativeX, relativeY){
  const videoRect = remoteVideo.getBoundingClientRect();
  // 视频标签大小
  let displayWidth = videoRect.width ; 
  let displayHeight=videoRect.height ;

  //计算视频实际大小
  calculateSize(); // 
  if(displayWidth>targetWidth){
      //减去坐左边的黑边
      relativeX=relativeX-(displayWidth-targetWidth)/2;
      if(relativeX<0){
          relativeX=0;
      }
      displayWidth=targetWidth;
  }
  if(displayHeight>targetHeight){
    //减去上遍的黑边
      relativeY=relativeY-(displayHeight-targetHeight)/2;
      if(relativeY<0){
          relativeY=0;
      }
      displayHeight=targetHeight;
  }
  //修复真实的坐标映射
  remoteX = Math.round((relativeX) * (videoWidth /displayWidth));
  remoteY = Math.round((relativeY)* (videoHeight / displayHeight));
  return {remoteX, remoteY};
}

