import{i as ce,n as _,aN as de,a8 as me,d as I,a as ve,b as J,m as fe,e as te,h as b,u as he,v as ne,w as L,f as le,x as pe,r as h,t as K,aa as se,aO as ae,ab as F,H as x,I as w,D as g,C as Y,Y as ge,E as W,L as O,a3 as ye,ac as Q,ad as q,O as A,G as R,M as X}from"./index-BsznBmGz.js";import{a as $e}from"./bindings-DYgdjGOb.js";import{N as Z,B as we}from"./BindingSelector-B5cb4F4t.js";import{u as be}from"./use-message-Di2NNvCu.js";import{u as _e}from"./composables-DlsQSUjP.js";import{a as ee,N as je,b as xe}from"./Spin-DD6t4OuR.js";import{N as Ne}from"./DatePicker-C57kEK-9.js";import{N as Se}from"./Switch-DLSTPOsI.js";import{N as Ce}from"./Input-Bz08ueo2.js";import{N as ke}from"./DataTable-orZ5Bh6D.js";import{_ as Te}from"./_plugin-vue_export-helper-7h7LVAwr.js";import"./Tooltip-Bzhmub-g.js";import"./get-qhDypvqw.js";import"./Dropdown-XzCU038w.js";import"./prop-NnGblK-3.js";function Ee(l,n){const o=ce(de,null);return _(()=>l.hljs||(o==null?void 0:o.mergedHljsRef.value))}function Re(l){const{textColor2:n,fontSize:o,fontWeightStrong:p,textColor3:m}=l;return{textColor:n,fontSize:o,fontWeightStrong:p,"mono-3":"#a0a1a7","hue-1":"#0184bb","hue-2":"#4078f2","hue-3":"#a626a4","hue-4":"#50a14f","hue-5":"#e45649","hue-5-2":"#c91243","hue-6":"#986801","hue-6-2":"#c18401",lineNumberTextColor:m}}const Le={common:me,self:Re},ze=I([ve("code",`
 font-size: var(--n-font-size);
 font-family: var(--n-font-family);
 `,[J("show-line-numbers",`
 display: flex;
 `),fe("line-numbers",`
 user-select: none;
 padding-right: 12px;
 text-align: right;
 transition: color .3s var(--n-bezier);
 color: var(--n-line-number-text-color);
 `),J("word-wrap",[I("pre",`
 white-space: pre-wrap;
 word-break: break-all;
 `)]),I("pre",`
 margin: 0;
 line-height: inherit;
 font-size: inherit;
 font-family: inherit;
 `),I("[class^=hljs]",`
 color: var(--n-text-color);
 transition: 
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `)]),({props:l})=>{const n=`${l.bPrefix}code`;return[`${n} .hljs-comment,
 ${n} .hljs-quote {
 color: var(--n-mono-3);
 font-style: italic;
 }`,`${n} .hljs-doctag,
 ${n} .hljs-keyword,
 ${n} .hljs-formula {
 color: var(--n-hue-3);
 }`,`${n} .hljs-section,
 ${n} .hljs-name,
 ${n} .hljs-selector-tag,
 ${n} .hljs-deletion,
 ${n} .hljs-subst {
 color: var(--n-hue-5);
 }`,`${n} .hljs-literal {
 color: var(--n-hue-1);
 }`,`${n} .hljs-string,
 ${n} .hljs-regexp,
 ${n} .hljs-addition,
 ${n} .hljs-attribute,
 ${n} .hljs-meta-string {
 color: var(--n-hue-4);
 }`,`${n} .hljs-built_in,
 ${n} .hljs-class .hljs-title {
 color: var(--n-hue-6-2);
 }`,`${n} .hljs-attr,
 ${n} .hljs-variable,
 ${n} .hljs-template-variable,
 ${n} .hljs-type,
 ${n} .hljs-selector-class,
 ${n} .hljs-selector-attr,
 ${n} .hljs-selector-pseudo,
 ${n} .hljs-number {
 color: var(--n-hue-6);
 }`,`${n} .hljs-symbol,
 ${n} .hljs-bullet,
 ${n} .hljs-link,
 ${n} .hljs-meta,
 ${n} .hljs-selector-id,
 ${n} .hljs-title {
 color: var(--n-hue-2);
 }`,`${n} .hljs-emphasis {
 font-style: italic;
 }`,`${n} .hljs-strong {
 font-weight: var(--n-font-weight-strong);
 }`,`${n} .hljs-link {
 text-decoration: underline;
 }`]}]),Ue=Object.assign(Object.assign({},le.props),{language:String,code:{type:String,default:""},trim:{type:Boolean,default:!0},hljs:Object,uri:Boolean,inline:Boolean,wordWrap:Boolean,showLineNumbers:Boolean,internalFontSize:Number,internalNoHighlight:Boolean}),He=te({name:"Code",props:Ue,setup(l,{slots:n}){const{internalNoHighlight:o}=l,{mergedClsPrefixRef:p,inlineThemeDisabled:m}=he(),r=h(null),c=o?{value:void 0}:Ee(l),N=(s,d,i)=>{const{value:f}=c;return!f||!(s&&f.getLanguage(s))?null:f.highlight(i?d.trim():d,{language:s}).value},u=_(()=>l.inline||l.wordWrap?!1:l.showLineNumbers),$=()=>{if(n.default)return;const{value:s}=r;if(!s)return;const{language:d}=l,i=l.uri?window.decodeURIComponent(l.code):l.code;if(d){const y=N(d,i,l.trim);if(y!==null){if(l.inline)s.innerHTML=y;else{const k=s.querySelector(".__code__");k&&s.removeChild(k);const C=document.createElement("pre");C.className="__code__",C.innerHTML=y,s.appendChild(C)}return}}if(l.inline){s.textContent=i;return}const f=s.querySelector(".__code__");if(f)f.textContent=i;else{const y=document.createElement("pre");y.className="__code__",y.textContent=i,s.innerHTML="",s.appendChild(y)}};ne($),L(K(l,"language"),$),L(K(l,"code"),$),o||L(c,$);const j=le("Code","-code",ze,Le,l,p),S=_(()=>{const{common:{cubicBezierEaseInOut:s,fontFamilyMono:d},self:{textColor:i,fontSize:f,fontWeightStrong:y,lineNumberTextColor:k,"mono-3":C,"hue-1":B,"hue-2":D,"hue-3":P,"hue-4":z,"hue-5":U,"hue-5-2":E,"hue-6":H,"hue-6-2":M}}=j.value,{internalFontSize:T}=l;return{"--n-font-size":T?`${T}px`:f,"--n-font-family":d,"--n-font-weight-strong":y,"--n-bezier":s,"--n-text-color":i,"--n-mono-3":C,"--n-hue-1":B,"--n-hue-2":D,"--n-hue-3":P,"--n-hue-4":z,"--n-hue-5":U,"--n-hue-5-2":E,"--n-hue-6":H,"--n-hue-6-2":M,"--n-line-number-text-color":k}}),v=m?pe("code",_(()=>`${l.internalFontSize||"a"}`),S,l):void 0;return{mergedClsPrefix:p,codeRef:r,mergedShowLineNumbers:u,lineNumbers:_(()=>{let s=1;const d=[];let i=!1;for(const f of l.code)f===`
`?(i=!0,d.push(s++)):i=!1;return i||d.push(s++),d.join(`
`)}),cssVars:m?void 0:S,themeClass:v==null?void 0:v.themeClass,onRender:v==null?void 0:v.onRender}},render(){var l,n;const{mergedClsPrefix:o,wordWrap:p,mergedShowLineNumbers:m,onRender:r}=this;return r==null||r(),b("code",{class:[`${o}-code`,this.themeClass,p&&`${o}-code--word-wrap`,m&&`${o}-code--show-line-numbers`],style:this.cssVars,ref:"codeRef"},m?b("pre",{class:`${o}-code__line-numbers`},this.lineNumbers):null,(n=(l=this.$slots).default)===null||n===void 0?void 0:n.call(l))}}),Ie=["danmaku","super_chat","super_chat_delete","gift","gift_combo","guard_buy","user_enter","user_follow","user_share","user_like","live_start","live_stop","room_change","user_blocked","online_rank_update","room_stats_update","battle","pk_visit_from_opponent","pk_visit_to_opponent","manual","unknown"];function Oe(l,n={}){const o=n.max??500,p=ae([]),m=h(!1),r=h(null),c=new EventSource(`/api/bindings/${l}/stream`);let N=!1;c.onopen=()=>{m.value=!0,r.value=null},c.onerror=()=>{m.value=!1,r.value="实时连接断开，正在自动重连"};for(const $ of Ie)c.addEventListener($,j=>{if(N)return;let S;try{S=JSON.parse(j.data)}catch{return}const v=[S,...p.value];p.value=v.length>o?v.slice(0,o):v});function u(){N=!0,c.close(),m.value=!1}return se(u),{events:p,connected:m,error:r,close:u}}const Be={class:"logs-page"},De={class:"page-header"},Pe={class:"filters"},Me={class:"realtime-toggle"},Ve={class:"search-row"},Fe={class:"clear-group"},We={key:0,class:"clear-result"},qe=te({__name:"Logs",setup(l){const n=$e(),o=be(),p=_e(),m=[{label:"全部",value:"all"},{label:"事件",value:"event"},{label:"动作",value:"action"}],r=h("all"),c=h(null),N=h([]),u=h(null),$=h(""),j=h(!0);async function S(){try{N.value=await q("GET","/api/meta/event-types")}catch(t){o.error(t instanceof A?t.message:"加载事件类型清单失败")}}const v=h([]),s=h(!1);async function d(){const t=n.current;if(!t){v.value=[];return}s.value=!0;try{const e=new URLSearchParams;r.value!=="all"&&e.set("kind",r.value),c.value&&e.set("eventType",c.value),u.value&&(e.set("since",new Date(u.value[0]).toISOString()),e.set("until",new Date(u.value[1]).toISOString()));const a=e.toString();v.value=await q("GET",`/api/bindings/${t.id}/activity${a?`?${a}`:""}`)}catch(e){o.error(e instanceof A?e.message:"加载历史日志失败")}finally{s.value=!1}}const i=ae(null);function f(){var t;(t=i.value)==null||t.close(),i.value=null}function y(t){f(),i.value=Oe(t)}L(()=>[n.currentId,j.value],([t,e])=>{e&&t!==null?y(t):f()},{immediate:!0}),L(()=>n.currentId,()=>void d(),{immediate:!0}),ne(()=>void S()),se(f);function k(t){return{key:`h-${t.id}`,time:t.occurredAt,type:t.kind==="action"?t.actionType:t.eventType,ruleName:t.ruleName||"-",user:t.userName||t.userUid||"-",detail:t.detail,realtime:!1}}function C(t){if(t&&typeof t=="object"&&"User"in t){const e=t.User;if(e!=null&&e.Username)return e.UID?`${e.Username}(${e.UID})`:e.Username}return"-"}function B(t){return{key:`s-${t.id}`,time:t.timestamp,type:t.type,ruleName:"-",user:C(t.payload),detail:t.payload,realtime:!0}}const D=_(()=>{var e;return j.value?(((e=i.value)==null?void 0:e.events.value)??[]).filter(()=>r.value!=="action").filter(a=>!c.value||a.type===c.value).map(B):[]}),P=_(()=>v.value.map(k)),z=_(()=>[...D.value,...P.value]),U=_(()=>{const t=$.value.trim().toLowerCase();return t?z.value.filter(e=>`${e.type} ${e.ruleName} ${e.user} ${JSON.stringify(e.detail)}`.toLowerCase().includes(t)):z.value}),E=h(new Set);function H(t){const e=new Set(E.value);e.has(t)?e.delete(t):e.add(t),E.value=e}const M=[{title:"时间",key:"time",width:190},{title:"类型",key:"type",width:170,render:t=>t.realtime?b("span",[b(xe,{size:"small",type:"success"},{default:()=>"实时"})," ",t.type]):t.type},{title:"规则名",key:"ruleName",width:140},{title:"用户",key:"user",width:160},{title:"详情",key:"detail",render:t=>E.value.has(t.key)?b("div",[b(O,{size:"tiny",text:!0,onClick:()=>H(t.key)},{default:()=>"收起"}),b(He,{code:JSON.stringify(t.detail,null,2),language:"json",style:"max-width: 640px; white-space: pre-wrap;"})]):b(O,{size:"tiny",text:!0,onClick:()=>H(t.key)},{default:()=>"查看详情"})}],T=h(!1),V=h(null);function oe(){return u.value?`${new Date(u.value[0]).toLocaleString()} 至 ${new Date(u.value[1]).toLocaleString()}`:"全部历史（未设置时间范围）"}function re(){const t=n.current;if(!t)return;const e=oe(),a=r.value!=="all"||!!c.value,G=[`确定要清除「${e}」范围内的业务日志吗？这是真的从数据库删除，不可恢复。`];a&&G.push("注意：清除只按时间范围执行，不支持按类型/关键词筛选——即使你已经用类型或关键词筛出了一部分记录，清除的仍是这个时间范围内的全部日志，不局限于当前筛选结果。"),p.warning({title:"清除业务日志",content:()=>b("div",G.map(ue=>b("p",{style:"margin: 4px 0"},ue))),positiveText:"清除",negativeText:"取消",onPositiveClick:()=>void ie(t.id)})}async function ie(t){T.value=!0;try{const e=new URLSearchParams;u.value?(e.set("since",new Date(u.value[0]).toISOString()),e.set("until",new Date(u.value[1]).toISOString())):e.set("all","1");const a=await q("DELETE",`/api/bindings/${t}/activity?${e.toString()}`);V.value=a.deleted,o.success(`已清除 ${a.deleted} 条业务日志`),await d()}catch(e){o.error(e instanceof A?e.message:"清除失败")}finally{T.value=!1}}return(t,e)=>(R(),F("div",Be,[x("div",De,[e[5]||(e[5]=x("h2",null,"日志",-1)),w(we)]),g(n).current?(R(),F(ge,{key:1},[x("div",Pe,[w(g(Z),{value:r.value,"onUpdate:value":e[0]||(e[0]=a=>r.value=a),options:m,style:{width:"100px"}},null,8,["value"]),w(g(Z),{value:c.value,"onUpdate:value":e[1]||(e[1]=a=>c.value=a),options:N.value,clearable:"",placeholder:"全部类型",style:{width:"180px"}},null,8,["value","options"]),w(g(Ne),{value:u.value,"onUpdate:value":e[2]||(e[2]=a=>u.value=a),type:"datetimerange",clearable:"",style:{width:"380px"}},null,8,["value"]),w(g(O),{type:"primary",onClick:d},{default:W(()=>[...e[6]||(e[6]=[X("查询",-1)])]),_:1}),x("div",Me,[w(g(Se),{value:j.value,"onUpdate:value":e[3]||(e[3]=a=>j.value=a)},null,8,["value"]),e[7]||(e[7]=x("span",null,"实时",-1))])]),x("div",Ve,[w(g(Ce),{value:$.value,"onUpdate:value":e[4]||(e[4]=a=>$.value=a),placeholder:"按关键词搜索",style:{width:"260px"}},null,8,["value"]),e[9]||(e[9]=x("span",{class:"hint"},"只在已加载的记录里搜，不检索全部历史",-1)),x("div",Fe,[V.value!==null?(R(),F("span",We," 上次清除了 "+ye(V.value)+" 条 ",1)):Q("",!0),w(g(O),{type:"error",loading:T.value,onClick:re},{default:W(()=>[...e[8]||(e[8]=[X(" 清除 ",-1)])]),_:1},8,["loading"])])]),w(g(je),{show:s.value},{default:W(()=>[w(g(ke),{columns:M,data:U.value,"row-key":a=>a.key,bordered:!1,size:"small"},null,8,["data","row-key"]),U.value.length===0?(R(),Y(g(ee),{key:0,description:"没有符合条件的记录",size:"small"})):Q("",!0)]),_:1},8,["show"])],64)):(R(),Y(g(ee),{key:0,description:"请先在顶部选择一个直播间"}))]))}}),rt=Te(qe,[["__scopeId","data-v-53035a99"]]);export{rt as default};
