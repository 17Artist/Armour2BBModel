'use strict';
(() => {
  const $ = id => document.getElementById(id);
  const state = { worker: null, ready: false, busy: false, queued: null, file: null, data: null, model: null, selectedSlot: null, id: 0, bootTimer: null, jobTimer: null };
  const slots = [{code:'HEAD',label:'头'},{code:'BODY',label:'上半身'},{code:'LEGS',label:'下半身'},{code:'FEET',label:'鞋'},{code:'DECORATION',label:'装饰'}];
  const names = { Head:'头部',Body:'身体',Torso:'躯干',LeftArm:'左臂',RightArm:'右臂',LeftForeArm:'左前臂',RightForeArm:'右前臂',LeftLeg:'左腿',RightLeg:'右腿',LeftForeLeg:'左小腿',RightForeLeg:'右小腿',LeftWing:'左翼',RightWing:'右翼',LeftPhalanx:'左翼末节',RightPhalanx:'右翼末节',LeftFoot:'左脚',RightFoot:'右脚',Skirt:'裙摆',Sword:'剑',Pickaxe:'镐',Axe:'斧',Shovel:'锹',Hoe:'锄',Shield:'盾牌',ShieldBlocking:'盾牌格挡',Trident:'三叉戟',TridentThrowing:'三叉戟投掷',Bow:'弓',BowFrame0:'弓 · 帧 0',BowFrame1:'弓 · 帧 1',BowFrame2:'弓 · 帧 2',BowFrame3:'弓 · 帧 3',Arrow:'箭',FishingRod:'钓竿',FishingRodCast:'钓竿抛线',FishingHook:'鱼钩',Backpack:'背包',Boat:'船',LeftPaddle:'左船桨',RightPaddle:'右船桨',Minecart:'矿车',Multiblock:'多方块',Item:'物品',Block:'方块',Part:'部件' };
  const types = {head:'头部',chest:'上装',legs:'下装',feet:'鞋靴',wings:'翅膀',outfit:'全身时装',sword:'剑',bow:'弓',arrow:'箭',item:'物品',block:'方块',skirt:'裙装',hand:'手持物品',tool:'工具'};
  let preview = null;
  try { preview = new ModelPreview($('preview-canvas')); }
  catch { showNotice('当前浏览器无法启用三维预览。转换、分类检查和下载仍可使用。'); }

  function status(message, type = '') {
    $('status-text').textContent = message;
    $('status-message').className = `status-message ${type}`;
    $('status-indicator').className = `status-indicator ${type === 'loading' ? 'loading' : ''}`;
  }
  function showNotice(message) { $('preview-notice').textContent=message; $('preview-notice').hidden=!message; }
  function formatNumber(value) { return Number.isFinite(value) ? new Intl.NumberFormat('zh-CN').format(value) : '—'; }
  function formatBytes(value) { return value < 1024 ? `${value} B` : value < 1048576 ? `${(value/1024).toFixed(1)} KB` : `${(value/1048576).toFixed(1)} MB`; }
  function busy(value) {
    state.busy=value;
    $('processing-overlay').hidden=!value;
    $('drop-zone').disabled=value;
    $('keep-coincident-faces').disabled=value;
    $('clear-button').disabled=value;
    $('download-button').disabled=value || !state.data;
    $('download-slot-button').disabled=value || !state.data || !state.selectedSlot;
    $('model-view').setAttribute('aria-busy',String(value));
    if(value) setTab('model');
  }
  function clearResult() {
    state.data=null;state.model=null;state.selectedSlot=null;
    ['cube-count','voxel-count','reduction','skin-type','skin-version','elapsed','animation-count'].forEach(id=>$(id).textContent='—');
    $('model-name').textContent='尚未导入模型';
    $('result-badge').textContent='等待导入';$('result-badge').className='small-label';
    $('reduction-bar').style.width='0';$('part-list').replaceChildren();$('group-count').textContent='0';
    $('structure-empty').hidden=false;$('empty-preview').hidden=false;
    $('texture-image').hidden=true;$('texture-image').removeAttribute('src');$('texture-empty').hidden=false;$('texture-caption').textContent='';
    $('download-button').disabled=true;$('export-hint').textContent='转换完成后可下载';
    $('download-slot-button').disabled=true;$('download-slot-label').textContent='下载当前分类';$('slot-export-hint').textContent='选择一个分类后，可单独下载';
    $('preview-selection').textContent='全部分类';
    $('result-warnings').replaceChildren();$('warnings-section').hidden=true;
    $('classification-notes').replaceChildren();$('classification-notes-section').hidden=true;
    $('structure-foot').textContent='每个方块只归属一个分类';
    $('step-import').className='active';$('step-check').className='';$('step-export').className='';
    if(preview) preview.clear();
    setTab('model');
  }
  function boot() {
    if(state.worker) state.worker.terminate();
    clearTimeout(state.bootTimer);clearTimeout(state.jobTimer);
    state.ready=false;$('retry-button').hidden=true;
    status('正在加载转换引擎…','loading');
    try {
      state.worker=new Worker('converter-worker.js');
      state.worker.onmessage=receive;
      state.worker.onerror=event=>engineFailed(event.message || '转换线程启动失败');
      state.bootTimer=setTimeout(()=>engineFailed('引擎加载超时，请检查网络并重试'),60000);
    } catch(error) { engineFailed(error.message); }
  }
  function engineFailed(message) {
    clearTimeout(state.bootTimer);clearTimeout(state.jobTimer);
    state.ready=false;state.queued=null;
    if(state.worker) { state.worker.terminate();state.worker=null; }
    busy(false);status(`引擎加载失败：${message}`,'error');$('retry-button').hidden=false;
  }
  async function receive(event) {
    const data=event.data;
    if(data.type==='ready') {
      clearTimeout(state.bootTimer);state.ready=true;$('retry-button').hidden=true;
      if(state.queued) { const file=state.queued;state.queued=null;convert(file); }
      else status('请选择时装文件','success');
      return;
    }
    if(data.type==='engine-error') { engineFailed(data.error);return; }
    if(data.id!==state.id) return;
    if(data.type==='phase') { $('processing-text').textContent='正在转换时装…';status(data.message,'loading');return; }
    clearTimeout(state.jobTimer);
    if(data.type==='error') { busy(false);status(data.error,'error');$('result-badge').textContent='转换失败';return; }
    if(data.type==='result') {
      try {
        const model=JSON.parse(new TextDecoder().decode(data.data));
        if(!Array.isArray(model.elements)) throw new Error('模型数据不完整');
        state.model=model;state.data=data.data;
        populate(model,data);
        busy(false);status('转换完成','success');
        const id=state.id;
        if(preview) {
          try { await preview.setModel(model); }
          catch(error) { if(id===state.id) showNotice(`三维预览暂不可用：${error.message}。仍可下载完整模型。`); }
        }
      } catch(error) { state.data=null;busy(false);status(`读取转换结果失败：${error.message}`,'error'); }
    }
  }
  async function convert(file) {
    if(state.busy) return;
    if(!/\.(armour|awsk)$/i.test(file.name)) { status('请选择 .armour 或 .awsk 时装文件','error');return; }
    if(file.size===0) { status('文件为空，请重新选择','error');return; }
    if(file.size>64*1024*1024) { status('文件超过 64 MiB，请使用命令行转换','error');return; }
    state.file=file;$('selected-file').hidden=false;$('filename').textContent=file.name;$('filesize').textContent=formatBytes(file.size);
    clearResult();if(preview) showNotice('');
    if(!state.ready) { state.queued=file;status('文件已选中，等待转换引擎就绪…','loading');return; }
    const keepCoincidentFaces=$('keep-coincident-faces').checked===true;
    busy(true);const id=++state.id;
    $('result-badge').textContent='转换中';status('正在读取时装文件…','loading');
    try {
      const buffer=await file.arrayBuffer();
      if(id!==state.id) return;
      state.jobTimer=setTimeout(()=>{
        state.worker.terminate();state.id++;busy(false);
        boot();status('转换已超时，正在重新准备引擎。可尝试较小的文件。','error');
      },180000);
      state.worker.postMessage({type:'convert',id,buffer,keepCoincidentFaces},[buffer]);
    } catch(error) { if(id===state.id) { busy(false);status(`读取文件失败：${error.message}`,'error'); } }
  }
  function populate(model,result) {
    const counts=countSlots(model);
    const stats=result.stats || {};
    const input=stats.inputVoxels,output=model.elements.length;
    const ratio=Number.isFinite(input) && input>0 ? Math.max(0,(1-output/input)*100) : null;
    $('cube-count').textContent=formatNumber(output);$('voxel-count').textContent=formatNumber(input);
    $('reduction').textContent=ratio===null?'—':`${ratio.toFixed(1)}%`;$('reduction-bar').style.width=`${ratio || 0}%`;
    const skinType=String(result.skinType || '').split(':').pop();
    $('skin-type').textContent=types[skinType] || skinType || '未知';
    $('skin-type').title=result.skinType || '';
    $('skin-version').textContent=result.metadata?.version!=null?`v${result.metadata.version}`:'—';
    const elapsed=stats.elapsedMs ?? result.elapsedMs;
    $('elapsed').textContent=Number.isFinite(elapsed)?(elapsed<1000?`${Math.round(elapsed)} ms`:`${(elapsed/1000).toFixed(2)} s`):'—';
    $('animation-count').textContent=formatNumber((model.animations || []).length);
    $('model-name').textContent=model.name || state.file.name.replace(/\.[^.]+$/,'');
    const warnings=Array.isArray(result.warnings)?result.warnings:[];
    $('result-warnings').replaceChildren();warnings.forEach(message=>{const item=document.createElement('li');item.textContent=String(message);$('result-warnings').append(item);});
    $('warnings-section').hidden=warnings.length===0;
    $('result-badge').textContent='已完成';$('result-badge').className='small-label success';
    $('empty-preview').hidden=true;
    const texture=model.textures?.[0];
    if(texture?.source) {
      $('texture-image').src=texture.source;$('texture-image').hidden=false;$('texture-empty').hidden=true;
      $('texture-caption').textContent=`${texture.width} × ${texture.height} px · ${(model.textures || []).length} 张图集`;
    } else $('texture-empty').textContent='模型没有纹理图集。';
    const notes=Array.isArray(model.classification_notes)?model.classification_notes:[];
    $('classification-notes').replaceChildren();notes.forEach(message=>{const item=document.createElement('li');item.textContent=String(message);$('classification-notes').append(item);});
    $('classification-notes-section').hidden=notes.length===0;
    const sources=collectSources(model);
    $('group-count').textContent=String(slots.length);$('structure-empty').hidden=true;
    const list=$('part-list');list.replaceChildren();
    addSlotButton(list,null,'全部分类','ALL',output,true);
    slots.forEach(slot=>{
      const section=document.createElement('div');section.className='slot-section';
      addSlotButton(section,slot.code,slot.label,slot.code,counts.get(slot.code).size,false);
      const records=sources.filter(source=>source.slot===slot.code);
      if(records.length) addSourceDetails(section,records);
      list.append(section);
    });
    $('structure-foot').textContent=`5 类互斥统计 · 合计 ${formatNumber(output)} 个方块`;
    $('export-hint').textContent=`${formatBytes(state.data.byteLength)} · 包含全部分类`;
    $('step-import').className='complete';$('step-check').className='active';$('step-export').className='';
  }
  function countSlots(model) {
    const counts=new Map(slots.map(slot=>[slot.code,new Set()]));const seen=new Set();
    model.elements.forEach(element=>{
      if(!counts.has(element.costume_slot)) throw new Error('模型缺少有效的时装分类信息，请重新加载转换引擎');
      if(!element.uuid || seen.has(element.uuid)) throw new Error('模型包含重复或缺失的方块标识');
      seen.add(element.uuid);counts.get(element.costume_slot).add(element.uuid);
    });
    return counts;
  }
  function collectSources(model) {
    const groups=new Map((model.groups || []).map(group=>[group.uuid,group]));
    const elements=new Map(model.elements.map(element=>[element.uuid,element]));
    const seenNodes=new Set();const records=[];
    function walk(node) {
      if(typeof node==='string' || !node || seenNodes.has(node.uuid)) return;
      seenNodes.add(node.uuid);
      const group=groups.get(node.uuid) || node;
      if(group.source_part || group.source_name || Number.isInteger(group.source_index)) {
        const direct=new Set((node.children || []).filter(child=>typeof child==='string' && elements.has(child)));
        slots.forEach(slot=>{
          const count=[...direct].filter(id=>elements.get(id).costume_slot===slot.code).length;
          if(count) records.push({...group,slot:slot.code,directCount:count});
        });
      }
      (node.children || []).forEach(walk);
    }
    (model.outliner || []).forEach(node=>walk(node));
    return records;
  }
  function addSlotButton(list,slot,label,code,count,selected) {
    const button=document.createElement('button');button.className=`part-button${selected?' selected':''}`;
    button.type='button';button.dataset.slot=slot || 'ALL';button.setAttribute('aria-pressed',String(selected));button.disabled=count===0;
    const svg=document.createElementNS('http://www.w3.org/2000/svg','svg');svg.classList.add('icon');svg.setAttribute('aria-hidden','true');
    const use=document.createElementNS('http://www.w3.org/2000/svg','use');use.setAttribute('href',slot?'#i-cube':'#i-layers');svg.append(use);
    const text=document.createElement('div');const main=document.createElement('span');main.className='part-label';main.textContent=label;
    const secondary=document.createElement('span');secondary.className='part-code';secondary.textContent=code;text.append(main,secondary);
    const amount=document.createElement('span');amount.className='part-count';amount.textContent=formatNumber(count);
    button.append(svg,text,amount);button.addEventListener('click',()=>{
      if(button.disabled) return;
      markSelection(button);state.selectedSlot=slot;
      $('download-slot-button').disabled=state.busy || !state.data || !slot;
      $('download-slot-label').textContent=slot?`下载${label}`:'下载当前分类';
      $('slot-export-hint').textContent=slot?`${label} · ${formatNumber(count)} 个方块 · 保留骨骼和图集`:'选择一个分类后，可单独下载';
      if(preview) preview.selectSlot(slot);
      $('preview-selection').textContent=label;setTab('model');
    });list.append(button);
  }
  function markSelection(button) {
    $('part-list').querySelectorAll('button').forEach(item=>{const active=item===button;item.classList.toggle('selected',active);item.setAttribute('aria-pressed',String(active));});
  }
  function addSourceDetails(section,records) {
    const details=document.createElement('details');details.className='source-details';
    const summary=document.createElement('summary');summary.textContent=`查看 ${records.length} 个来源记录`;details.append(summary);
    records.forEach(source=>{
      const row=document.createElement('button');row.type='button';row.className='source-row';row.dataset.source=source.uuid;row.setAttribute('aria-pressed','false');
      const attachment=source.attachment || '';const equipment=Number.isInteger(source.source_equipment) && source.source_equipment>=0?source.source_equipment+1:null;
      const index=Number.isInteger(source.source_index) && source.source_index>=0?source.source_index+1:null;
      const name=source.source_name || (index===null?'未命名源部件':`源根部件 ${index}`);
      row.title=`源挂点：${attachment || '未指定'}\n源名称：${source.source_name || '未命名'}\n源部件：${source.source_part || '未指定'}${index===null?'':`\n源根部件序号：${index}`}${equipment===null?'':`\n套装子装备：${equipment}`}\n直接方块：${formatNumber(source.directCount)}`;
      const title=document.createElement('span');title.className='source-name';title.textContent=name;
      const count=document.createElement('span');count.className='source-direct-count';count.textContent=`直接 ${formatNumber(source.directCount)} 块`;
      const metadata=document.createElement('span');metadata.className='part-source';metadata.textContent=[attachment?`${names[attachment] || attachment}挂点 (${attachment})`:'',equipment===null?'':`子装备 ${equipment}`,source.source_part].filter(Boolean).join(' · ');
      row.append(title,count,metadata);row.setAttribute('aria-label',`单独预览来源：${name}，直接 ${formatNumber(source.directCount)} 个方块`);
      row.addEventListener('click',()=>{
        markSelection(row);state.selectedSlot=null;$('download-slot-button').disabled=true;
        $('download-slot-label').textContent='下载当前分类';$('slot-export-hint').textContent='当前为来源预览，请选择分类后下载';
        if(preview) preview.select(source.uuid);$('preview-selection').textContent=`${name} · 来源`;setTab('model');
      });details.append(row);
    });section.append(details);
  }
  function cropModelToSlot(model,slot) {
    const category=slots.find(item=>item.code===slot);if(!category) throw new Error('请选择有效的时装分类');
    const elements=model.elements.filter(element=>element.costume_slot===slot);
    if(!elements.length) throw new Error('当前分类没有可导出的方块');
    const elementIds=new Set(elements.map(element=>element.uuid)),reachableGroups=new Map(),referenced=new Set();
    function trim(node) {
      if(typeof node==='string'){if(!elementIds.has(node) || referenced.has(node))return null;referenced.add(node);return node;}
      if(!node)return null;
      const children=(node.children || []).map(trim).filter(child=>child!==null);
      if(!children.length)return null;
      const result={...node,children};reachableGroups.set(node.uuid,result);return result;
    }
    const outliner=(model.outliner || []).map(trim).filter(node=>node!==null);
    if(referenced.size!==elementIds.size) throw new Error('当前分类的模型层级不完整，无法单独导出');
    const groups=(model.groups || []).filter(group=>reachableGroups.has(group.uuid)).map(group=>({...group,children:reachableGroups.get(group.uuid).children.map(child=>typeof child==='string'?child:child.uuid)}));
    const animations=(model.animations || []).map(animation=>{
      const animators=Object.fromEntries(Object.entries(animation.animators || {}).filter(([uuid])=>reachableGroups.has(uuid)));
      return Object.keys(animators).length?{...animation,animators}:null;
    }).filter(animation=>animation!==null);
    const result={...model,name:`${model.name || '时装模型'}-${category.label}`,elements,groups,outliner,animations};
    delete result.surface_deduplicated_texels;
    return result;
  }
  function downloadData(data,name) {
    const url=URL.createObjectURL(new Blob([data],{type:'application/json'}));const anchor=document.createElement('a');
    anchor.href=url;anchor.download=name;document.body.append(anchor);anchor.click();anchor.remove();setTimeout(()=>URL.revokeObjectURL(url),30000);
    status(`已准备下载 ${name}`,'success');$('step-check').className='complete';$('step-export').className='active';
  }
  function setTab(tab) {
    const model=tab==='model';
    $('model-view').hidden=!model;$('texture-view').hidden=model;
    ['model','texture'].forEach(name=>{const button=$(`${name}-tab`);const selected=name===tab;button.classList.toggle('selected',selected);button.setAttribute('aria-selected',String(selected));button.tabIndex=selected?0:-1;});
    $('preview-toolbar').hidden=!model;
    if(model && preview) preview.draw();
  }
  $('drop-zone').addEventListener('click',()=>$('file-input').click());
  $('file-input').addEventListener('change',()=>{if($('file-input').files[0]) convert($('file-input').files[0]);$('file-input').value='';});
  let dragDepth=0;
  $('drop-zone').addEventListener('dragenter',event=>{event.preventDefault();dragDepth++;if(!state.busy) $('drop-zone').classList.add('over');});
  $('drop-zone').addEventListener('dragover',event=>event.preventDefault());
  $('drop-zone').addEventListener('dragleave',()=>{if(--dragDepth<=0) {dragDepth=0;$('drop-zone').classList.remove('over');}});
  $('drop-zone').addEventListener('drop',event=>{event.preventDefault();dragDepth=0;$('drop-zone').classList.remove('over');if(state.busy) return;const files=event.dataTransfer.files;if(files.length>1) status('请每次导入一个时装文件','error');else if(files[0]) convert(files[0]);});
  window.addEventListener('dragover',event=>event.preventDefault());window.addEventListener('drop',event=>event.preventDefault());
  $('clear-button').addEventListener('click',()=>{state.id++;state.file=null;state.queued=null;$('selected-file').hidden=true;clearResult();status(state.ready?'请选择时装文件':'正在加载转换引擎…',state.ready?'success':'loading');});
  $('retry-button').addEventListener('click',boot);
  $('cancel-button').addEventListener('click',()=>{state.id++;state.queued=null;clearTimeout(state.jobTimer);busy(false);clearResult();boot();status('已取消转换，正在重新准备引擎…','loading');});
  $('download-button').addEventListener('click',()=>{
    if(state.busy || !state.data) return;
    downloadData(state.data,state.file.name.replace(/\.[^.]+$/,'')+'.bbmodel');
  });
  $('download-slot-button').addEventListener('click',()=>{
    if(state.busy || !state.data || !state.model || !state.selectedSlot)return;
    try {
      const model=cropModelToSlot(state.model,state.selectedSlot),category=slots.find(slot=>slot.code===state.selectedSlot);
      downloadData(JSON.stringify(model),`${state.file.name.replace(/\.[^.]+$/,'')}-${category.label}.bbmodel`);
    } catch(error) {status(`分类导出失败：${error.message}`,'error');}
  });
  ['model','texture'].forEach(name=>{
    const button=$(`${name}-tab`);button.addEventListener('click',()=>setTab(name));
    button.addEventListener('keydown',event=>{if(['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();const tab=event.key==='Home'?'model':event.key==='End'?'texture':name==='model'?'texture':'model';setTab(tab);$(`${tab}-tab`).focus();}});
  });
  $('rotate-left').onclick=()=>preview?.rotate(-.25,0);$('rotate-right').onclick=()=>preview?.rotate(.25,0);
  $('zoom-in').onclick=()=>preview?.zoom(.85);$('zoom-out').onclick=()=>preview?.zoom(1.18);$('reset-view').onclick=()=>preview?.reset();
  $('grid-button').onclick=()=>{const enabled=$('grid-button').getAttribute('aria-pressed')!=='true';$('grid-button').setAttribute('aria-pressed',String(enabled));$('grid-button').classList.toggle('active',enabled);preview?.grid(enabled);};
  $('help-button').onclick=()=>$('help-dialog').showModal();$('close-help').onclick=()=>$('help-dialog').close();$('help-done').onclick=()=>$('help-dialog').close();
  $('help-dialog').addEventListener('click',event=>{if(event.target===$('help-dialog')){const rect=$('help-dialog').getBoundingClientRect();if(event.clientX<rect.left || event.clientX>rect.right || event.clientY<rect.top || event.clientY>rect.bottom) $('help-dialog').close();}});
  boot();
})();
