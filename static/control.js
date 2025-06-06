
var videoObj = document.getElementById('remoteVideo');
var isCanvas=false;
if (document.getElementById('webglCanvas')) {
   videoObj = document.getElementById('webglCanvas');
   isCanvas=true;
}

// 指针按下（兼容鼠标、触摸）
let isPointerDown = false;
var startX=0;
var startY=0;
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
  mouseClick('panstart', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 2);
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
  mouseClick('pan', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 2);

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
    console.log(`pos`, pos);

  
    if(Math.abs(clientX-startX)  < touchNum&&  Math.abs(clientY  -startY ) < touchNum || !isPointerDown ){
      startX=0;
      startY=0; 
      mouseClick('left', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 2);
      isPointerDown = false;
      return; // 忽略点击事件，防止误触
    }
    isPointerDown = false;
    mouseClick('panend', Number.isNaN(pos.remoteX) ? 0:pos.remoteX,Number.isNaN(pos.remoteY)?0:pos.remoteY, 2);

}






function fixXy( relativeX, relativeY){
  const videoRect = remoteVideo.getBoundingClientRect();


  console.log("relativeX:"+ relativeX);
  console.log("relativeY:"+ relativeY);


  // 5. 计算实际视频区域（剔除黑边）
  let displayWidth =0;
  let displayHeight=0;
  if (!isCanvas) {
     displayWidth = videoRect.width ;  // 显示宽度（物理像素）
     displayHeight = videoRect.height ; // 显示高度（物理像素）
    calculateSize(); // 
    if(displayWidth>targetWidth){
        relativeX=relativeX-(displayWidth-targetWidth)/2;
        if(relativeX<0){
            relativeX=0;
        }
        displayWidth=targetWidth;
    }
    if(displayHeight>targetHeight){
        relativeY=relativeY-(displayHeight-targetHeight)/2;
        if(relativeY<0){
            relativeY=0;
        }
        displayHeight=targetHeight;
    }
  }else{
     displayWidth = targetWidth ;  // 显示宽度（物理像素）
     displayHeight =targetHeight ; // 显示高度（物理像素）
  }


  console.log("displayWidth"+ displayWidth );
  console.log("displayHeight"+ displayHeight);
  console.log("nativeWidth"+ nativeWidth);
  console.log("nativeHeight"+ nativeHeight);
  // 6. 映射到远程屏幕坐标
   remoteX = Math.round((relativeX) * (nativeWidth /displayWidth));
   remoteY = Math.round((relativeY)* (nativeHeight / displayHeight));

  return {remoteX, remoteY};
}

if (isCanvas){
  
  if (document.getElementById('videoBox')) {
   let  videoBox = document.getElementById('videoBox');
    videoBox.addEventListener('pointerup', (e) => {
      if (isPointerDown){
        clickUp(e,true);
      }
    });
 }
}