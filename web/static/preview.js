/* Small, dependency-free renderer for the converted Blockbench cube geometry. */
'use strict';
class ModelPreview {
  constructor(canvas) {
    this.canvas=canvas;this.gl=canvas.getContext('webgl',{alpha:true,antialias:true,preserveDrawingBuffer:false});
    if(!this.gl) throw new Error('WebGL 不可用');
    this.batches=[];this.textures=[];this.center=[0,12,0];this.radius=24;this.ground=0;this.selected=null;this.selectedSlot=null;this.selectionRevision=0;this.showGrid=true;this.generation=0;this.pending=false;this.model=null;
    this.initializeResources();
    this.reset();this.makeGrid();this.bindInput();
    this.resizeObserver=new ResizeObserver(()=>this.draw());this.resizeObserver.observe(canvas);
    canvas.addEventListener('webglcontextlost',event=>{event.preventDefault();this.lost=true;const notice=document.getElementById('preview-notice');notice.hidden=false;notice.textContent='三维预览已暂停，等待浏览器恢复。可下载完整模型。';});
    canvas.addEventListener('webglcontextrestored',async()=>{
      const notice=document.getElementById('preview-notice');
      const model=this.model,selection={uuid:this.selected,slot:this.selectedSlot,revision:this.selectionRevision};
      try {
        this.lost=false;this.textures=[];this.batches=[];this.initializeResources();this.makeGrid();if(model)await this.setModel(model);else this.draw();
        if(this.model===model && this.selectionRevision===selection.revision){if(selection.slot)this.selectSlot(selection.slot);else if(selection.uuid)this.select(selection.uuid);}
        notice.hidden=true;
      }
      catch {notice.hidden=false;notice.textContent='三维预览恢复失败。可下载完整模型，或重新加载页面。';}
    });
  }
  initializeResources() {
    this.program=this.makeProgram();this.locations={};
    ['uCenter','uYaw','uPitch','uDistance','uAspect','uNear','uFar','uTexture','uGrid'].forEach(name=>this.locations[name]=this.gl.getUniformLocation(this.program,name));
    this.position=this.gl.getAttribLocation(this.program,'aPosition');this.uv=this.gl.getAttribLocation(this.program,'aUV');this.shade=this.gl.getAttribLocation(this.program,'aShade');
    this.buffer=this.gl.createBuffer();this.gridBuffer=this.gl.createBuffer();
    this.gridTexture=this.gl.createTexture();this.gl.bindTexture(this.gl.TEXTURE_2D,this.gridTexture);this.gl.texImage2D(this.gl.TEXTURE_2D,0,this.gl.RGBA,1,1,0,this.gl.RGBA,this.gl.UNSIGNED_BYTE,new Uint8Array([255,255,255,255]));
  }
  makeProgram() {
    const gl=this.gl;
    const vertex=`attribute vec3 aPosition;attribute vec2 aUV;attribute float aShade;
      uniform vec3 uCenter;uniform vec2 uYaw;uniform vec2 uPitch;uniform float uDistance,uAspect,uNear,uFar;
      varying vec2 vUV;varying float vShade;
      void main(){vec3 p=aPosition-uCenter;float x=p.x*uYaw.x+p.z*uYaw.y;float z=-p.x*uYaw.y+p.z*uYaw.x;p.x=x;p.z=z;
        float y=p.y*uPitch.x-p.z*uPitch.y;z=p.y*uPitch.y+p.z*uPitch.x;p.y=y;p.z=z-uDistance;
        float f=2.41421356;gl_Position=vec4(p.x*f/uAspect,p.y*f,(uFar+uNear)/(uNear-uFar)*p.z+2.0*uFar*uNear/(uNear-uFar),-p.z);vUV=aUV;vShade=aShade;}`;
    const fragment=`precision mediump float;uniform sampler2D uTexture;uniform float uGrid;varying vec2 vUV;varying float vShade;
      void main(){if(uGrid>0.5){gl_FragColor=vec4(0.65,0.65,0.65,0.22);}else{vec4 c=texture2D(uTexture,vUV);if(c.a<0.01)discard;gl_FragColor=vec4(c.rgb*vShade,c.a);}}`;
    const compile=(type,source)=>{const shader=gl.createShader(type);gl.shaderSource(shader,source);gl.compileShader(shader);if(!gl.getShaderParameter(shader,gl.COMPILE_STATUS))throw new Error('预览着色器编译失败');return shader;};
    const program=gl.createProgram();const vs=compile(gl.VERTEX_SHADER,vertex),fs=compile(gl.FRAGMENT_SHADER,fragment);gl.attachShader(program,vs);gl.attachShader(program,fs);gl.linkProgram(program);gl.deleteShader(vs);gl.deleteShader(fs);
    if(!gl.getProgramParameter(program,gl.LINK_STATUS)) throw new Error('预览渲染器初始化失败');return program;
  }
  reset(){this.yaw=Math.PI+.5;this.pitch=.24;this.zoomFactor=1;this.draw();}
  rotate(horizontal,vertical){this.yaw+=horizontal;this.pitch=Math.max(-1.3,Math.min(1.3,this.pitch+vertical));this.draw();}
  zoom(factor){this.zoomFactor=Math.max(.3,Math.min(5,this.zoomFactor*factor));this.draw();}
  grid(enabled){this.showGrid=enabled;this.draw();}
  select(uuid){this.selectionRevision++;this.selected=uuid;this.selectedSlot=null;this.fitSelection();}
  selectSlot(slot){this.selectionRevision++;this.selectedSlot=slot;this.selected=null;this.fitSelection();}
  visibleBatches(){return this.batches.filter(batch=>(!this.selected || batch.ancestors.includes(this.selected)) && (!this.selectedSlot || batch.slot===this.selectedSlot));}
  fitSelection() {
    const visible=this.visibleBatches();
    if(!visible.length){this.draw();return false;}
    const min=[Infinity,Infinity,Infinity],max=[-Infinity,-Infinity,-Infinity];
    visible.forEach(batch=>batch.bounds.min.forEach((value,axis)=>{min[axis]=Math.min(min[axis],value);max[axis]=Math.max(max[axis],batch.bounds.max[axis]);}));
    this.center=min.map((value,axis)=>(value+max[axis])/2);
    this.radius=Math.max(2,Math.hypot(...min.map((value,axis)=>(max[axis]-value)/2)));
    this.ground=min[1]-.08;this.zoomFactor=1;this.makeGrid();this.draw();return true;
  }
  clear(){this.generation++;this.selectionRevision++;this.model=null;this.batches=[];this.selected=null;this.selectedSlot=null;this.center=[0,12,0];this.radius=24;this.ground=0;this.deleteTextures();this.makeGrid();this.reset();}
  deleteTextures(){this.textures.forEach(texture=>this.gl.deleteTexture(texture));this.textures=[];}
  bindInput() {
    const canvas=this.canvas;let pointer=null;
    canvas.addEventListener('pointerdown',event=>{if(event.button!==0)return;pointer={id:event.pointerId,x:event.clientX,y:event.clientY};canvas.setPointerCapture(event.pointerId);canvas.focus({preventScroll:true});});
    canvas.addEventListener('pointermove',event=>{if(!pointer || pointer.id!==event.pointerId)return;this.rotate((event.clientX-pointer.x)*.008,(event.clientY-pointer.y)*.008);pointer.x=event.clientX;pointer.y=event.clientY;});
    const end=()=>pointer=null;canvas.addEventListener('pointerup',end);canvas.addEventListener('pointercancel',end);canvas.addEventListener('lostpointercapture',end);
    canvas.addEventListener('wheel',event=>{event.preventDefault();this.zoom(Math.exp(Math.max(-100,Math.min(100,event.deltaY))*.002));},{passive:false});
    canvas.addEventListener('keydown',event=>{
      const commands={ArrowLeft:()=>this.rotate(-.12,0),ArrowRight:()=>this.rotate(.12,0),ArrowUp:()=>this.rotate(0,-.12),ArrowDown:()=>this.rotate(0,.12),'+':()=>this.zoom(.85),'=':()=>this.zoom(.85),'-':()=>this.zoom(1.18),Home:()=>this.reset()};
      if(commands[event.key] && !event.ctrlKey && !event.metaKey && !event.altKey){event.preventDefault();commands[event.key]();}
    });
  }
  static rotatePoint(point,transform) {
    const origin=transform.origin || [0,0,0],r=transform.rotation || [0,0,0],scale=transform.scale || [1,1,1];
    let p=point.map((value,i)=>(value-origin[i])*scale[i]);
    const rad=Math.PI/180;
    const rx=r[0]*rad,ry=r[1]*rad,rz=r[2]*rad;
    if(rx){const y=p[1]*Math.cos(rx)-p[2]*Math.sin(rx),z=p[1]*Math.sin(rx)+p[2]*Math.cos(rx);p[1]=y;p[2]=z;}
    if(ry){const x=p[0]*Math.cos(ry)+p[2]*Math.sin(ry),z=-p[0]*Math.sin(ry)+p[2]*Math.cos(ry);p[0]=x;p[2]=z;}
    if(rz){const x=p[0]*Math.cos(rz)-p[1]*Math.sin(rz),y=p[0]*Math.sin(rz)+p[1]*Math.cos(rz);p[0]=x;p[1]=y;}
    return p.map((value,i)=>value+origin[i]);
  }
  async setModel(model) {
    this.model=model;
    const generation=++this.generation;
    this.batches=[];this.selected=null;this.selectedSlot=null;this.deleteTextures();this.draw();
    if(model.elements.length>80000) throw new Error('模型超过 80,000 个方块，已跳过预览以节省浏览器内存');
    const gl=this.gl;
    const images=await Promise.all((model.textures || []).map(texture=>new Promise((resolve,reject)=>{
      const image=new Image();image.onload=()=>resolve(image);image.onerror=()=>reject(new Error('无法读取内嵌纹理'));image.src=texture.source;
    })));
    if(generation!==this.generation)return;
    images.forEach(image=>{const texture=gl.createTexture();gl.bindTexture(gl.TEXTURE_2D,texture);gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA,gl.RGBA,gl.UNSIGNED_BYTE,image);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MIN_FILTER,gl.NEAREST);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_MAG_FILTER,gl.NEAREST);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_S,gl.CLAMP_TO_EDGE);gl.texParameteri(gl.TEXTURE_2D,gl.TEXTURE_WRAP_T,gl.CLAMP_TO_EDGE);this.textures.push(texture);});
    if(!this.textures.length){const texture=gl.createTexture();gl.bindTexture(gl.TEXTURE_2D,texture);gl.texImage2D(gl.TEXTURE_2D,0,gl.RGBA,1,1,0,gl.RGBA,gl.UNSIGNED_BYTE,new Uint8Array([190,190,190,255]));this.textures.push(texture);}
    const elements=new Map(model.elements.map(element=>[element.uuid,element]));const groups=new Map((model.groups || []).map(group=>[group.uuid,group]));
    const placements=[];const used=new Set();
    function markHidden(node){if(typeof node==='string'){used.add(node);return;}(node?.children || []).forEach(markHidden);}
    function walk(node,chain=[],ancestors=[]) {
      if(typeof node==='string') {const element=elements.get(node);if(element&&!used.has(node)){used.add(node);placements.push({element,chain,ancestors});}return;}
      if(!node)return;
      const group=groups.get(node.uuid)||node;if(group.visibility===false){markHidden(node);return;}
      const nextChain=[...chain,group],nextAncestors=[...ancestors,node.uuid];
      (node.children || []).forEach(child=>walk(child,nextChain,nextAncestors));
    }
    (model.outliner || []).forEach(node=>walk(node));
    model.elements.forEach(element=>{if(!used.has(element.uuid))placements.push({element,chain:[],ancestors:[]});});
    const floats=[];
    const triangle=[0,1,2,0,2,3];
    placements.forEach(({element,chain,ancestors})=>{
      if(element.visibility===false)return;
      const [x0,y0,z0]=element.from,[x1,y1,z1]=element.to;
      // Blockbench face UVs run north from +X to -X, and south from -X to +X.
      const faces={north:[[x1,y1,z0],[x0,y1,z0],[x0,y0,z0],[x1,y0,z0]],south:[[x0,y1,z1],[x1,y1,z1],[x1,y0,z1],[x0,y0,z1]],east:[[x1,y1,z1],[x1,y1,z0],[x1,y0,z0],[x1,y0,z1]],west:[[x0,y1,z0],[x0,y1,z1],[x0,y0,z1],[x0,y0,z0]],up:[[x0,y1,z0],[x1,y1,z0],[x1,y1,z1],[x0,y1,z1]],down:[[x0,y0,z1],[x1,y0,z1],[x1,y0,z0],[x0,y0,z0]]};
      const transform=point=>{let p=point;if(Array.isArray(element.rotation))p=ModelPreview.rotatePoint(p,element);for(let index=chain.length-1;index>=0;index--)p=ModelPreview.rotatePoint(p,chain[index]);return p;};
      Object.entries(faces).forEach(([name,points])=>{
        const face=element.faces?.[name];if(!face || face.texture==null)return;
        const textureIndex=Number(face.texture);if(!this.textures[textureIndex])return;
        const texture=model.textures?.[textureIndex];const width=texture?.uv_width || model.resolution?.width || texture?.width || 16;const height=texture?.uv_height || model.resolution?.height || texture?.height || 16;
        const uv=face.uv || [0,0,1,1];let coords=[[uv[0]/width,uv[1]/height],[uv[2]/width,uv[1]/height],[uv[2]/width,uv[3]/height],[uv[0]/width,uv[3]/height]];
        const rotation=((face.rotation || 0)/90)%4;for(let turn=0;turn<rotation;turn++)coords.unshift(coords.pop());
        const shade=element.light_emission>0 || element.shade===false?1:({up:1,down:.6,north:.87,south:.87,east:.75,west:.75}[name]);
        const world=points.map(transform);const bounds={min:[Infinity,Infinity,Infinity],max:[-Infinity,-Infinity,-Infinity]};
        world.forEach(point=>point.forEach((value,axis)=>{bounds.min[axis]=Math.min(bounds.min[axis],value);bounds.max[axis]=Math.max(bounds.max[axis],value);}));
        const start=floats.length/6;triangle.forEach(index=>floats.push(...world[index],...coords[index],shade));
        const transparent=element.shade===false;
        const slot=element.costume_slot;
        const previous=this.batches[this.batches.length-1];const key=`${textureIndex}:${ancestors.join('/')}:${transparent}:${slot}`;
        // Glass faces stay separate so their camera depth can determine blend order.
        const center=[0,1,2].map(axis=>world.reduce((sum,point)=>sum+point[axis],0)/4);
        if(!transparent && previous?.key===key){previous.count+=6;bounds.min.forEach((value,axis)=>{previous.bounds.min[axis]=Math.min(previous.bounds.min[axis],value);previous.bounds.max[axis]=Math.max(previous.bounds.max[axis],bounds.max[axis]);});}
        else this.batches.push({key,start,count:6,texture:textureIndex,ancestors,transparent,center,slot,bounds});
      });
    });
    gl.bindBuffer(gl.ARRAY_BUFFER,this.buffer);gl.bufferData(gl.ARRAY_BUFFER,new Float32Array(floats),gl.STATIC_DRAW);
    if(!this.fitSelection()){this.center=[0,12,0];this.radius=24;this.ground=0;this.makeGrid();}this.reset();
  }
  makeGrid() {
    const values=[];const span=Math.max(16,Math.ceil(this.radius*1.5/8)*8);const step=Math.max(1,Math.ceil(span/32));
    const cx=Math.round(this.center[0]/step)*step,cz=Math.round(this.center[2]/step)*step;
    for(let n=-span;n<=span;n+=step){values.push(cx+n,this.ground,cz-span,0,0,1,cx+n,this.ground,cz+span,0,0,1,cx-span,this.ground,cz+n,0,0,1,cx+span,this.ground,cz+n,0,0,1);}
    this.gridCount=values.length/6;this.gl.bindBuffer(this.gl.ARRAY_BUFFER,this.gridBuffer);this.gl.bufferData(this.gl.ARRAY_BUFFER,new Float32Array(values),this.gl.STATIC_DRAW);
  }
  draw() {
    if(this.pending || this.lost || !this.program)return;this.pending=true;
    requestAnimationFrame(()=>{this.pending=false;this.render();});
  }
  render() {
    if(this.lost)return;
    const gl=this.gl,canvas=this.canvas,rect=canvas.getBoundingClientRect();if(!rect.width || !rect.height)return;
    const ratio=Math.min(window.devicePixelRatio || 1,2),width=Math.round(rect.width*ratio),height=Math.round(rect.height*ratio);
    if(canvas.width!==width || canvas.height!==height){canvas.width=width;canvas.height=height;}
    gl.viewport(0,0,width,height);gl.clearColor(27/255,27/255,29/255,1);gl.depthMask(true);gl.clear(gl.COLOR_BUFFER_BIT|gl.DEPTH_BUFFER_BIT);gl.enable(gl.DEPTH_TEST);gl.enable(gl.BLEND);gl.blendFuncSeparate(gl.SRC_ALPHA,gl.ONE_MINUS_SRC_ALPHA,gl.ONE,gl.ONE_MINUS_SRC_ALPHA);gl.useProgram(this.program);
    const aspect=width/height,distance=this.radius*2.8*this.zoomFactor/Math.min(aspect,1);
    gl.uniform3fv(this.locations.uCenter,this.center);gl.uniform2f(this.locations.uYaw,Math.cos(this.yaw),Math.sin(this.yaw));gl.uniform2f(this.locations.uPitch,Math.cos(this.pitch),Math.sin(this.pitch));gl.uniform1f(this.locations.uDistance,distance);gl.uniform1f(this.locations.uAspect,aspect);gl.uniform1f(this.locations.uNear,.1);gl.uniform1f(this.locations.uFar,Math.max(1000,distance+this.radius*10));gl.uniform1i(this.locations.uTexture,0);gl.activeTexture(gl.TEXTURE0);
    const attributes=buffer=>{gl.bindBuffer(gl.ARRAY_BUFFER,buffer);gl.enableVertexAttribArray(this.position);gl.vertexAttribPointer(this.position,3,gl.FLOAT,false,24,0);gl.enableVertexAttribArray(this.uv);gl.vertexAttribPointer(this.uv,2,gl.FLOAT,false,24,12);gl.enableVertexAttribArray(this.shade);gl.vertexAttribPointer(this.shade,1,gl.FLOAT,false,24,20);};
    if(this.showGrid){attributes(this.gridBuffer);gl.bindTexture(gl.TEXTURE_2D,this.gridTexture);gl.uniform1f(this.locations.uGrid,1);gl.depthMask(false);gl.drawArrays(gl.LINES,0,this.gridCount);gl.depthMask(true);}
    if(this.batches.length){
      attributes(this.buffer);gl.uniform1f(this.locations.uGrid,0);
      const visible=this.visibleBatches();
      const draw=batch=>{gl.bindTexture(gl.TEXTURE_2D,this.textures[batch.texture]);gl.drawArrays(gl.TRIANGLES,batch.start,batch.count);};
      // Opaque/masked pixels write depth. The fragment shader discards alpha-zero mask pixels
      // before they reach that depth write, so geometry behind a paint-NONE hole remains visible.
      visible.filter(batch=>!batch.transparent).forEach(draw);
      const cameraDepth=batch=>{const p=batch.center.map((value,index)=>value-this.center[index]);const z=-p[0]*Math.sin(this.yaw)+p[2]*Math.cos(this.yaw);return p[1]*Math.sin(this.pitch)+z*Math.cos(this.pitch);};
      const glass=visible.filter(batch=>batch.transparent).map(batch=>({batch,depth:cameraDepth(batch)})).sort((a,b)=>a.depth-b.depth);
      // Translucent surfaces use the opaque depth buffer but never block other glass surfaces.
      gl.depthMask(false);glass.forEach(item=>draw(item.batch));gl.depthMask(true);
    }
  }
}
