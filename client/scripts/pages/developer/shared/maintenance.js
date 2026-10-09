window.ManagementMaintenance=(()=>{
 const esc=value=>window.AdminShared?.escapeHTML?.(value)??String(value??'');
 async function challenge(ctx,action,title,text){
  return window.AdminShared.actionChallenge(ctx,action,title,text,'/developer/database/challenge');
 }
 async function guarded(ctx,{action,title,text,request}){
  const code=await challenge(ctx,action,title,text);
  if(!code)return null;
  return request(code);
 }
 async function requestWithChallenge(endpoint,{method='POST',body,challenge:code}){
  return window.API.request(endpoint,{method,body,headers:{'X-ElectriCredit-Challenge':code}});
 }
 async function downloadPOST(path,body,filename){
  const response=await fetch(`${window.API.getBaseURL()}${path}`,{method:'POST',headers:{'Content-Type':'application/json'},credentials:'include',body:JSON.stringify(body||{})});
  if(!response.ok){let message='Download failed.';try{const j=await response.json();message=j.error||message}catch{}throw new Error(message)}
  const blob=await response.blob(),url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=filename;document.body.appendChild(a);a.click();a.remove();setTimeout(()=>URL.revokeObjectURL(url),500)
 }
 return{challenge,guarded,requestWithChallenge,downloadPOST};
})();
