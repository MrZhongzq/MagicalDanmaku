import{bn as $t,aw as Et,aq as Ye,w as ye,bo as Kt,bp as Lt,o as Cn,bj as Ze,r as B,$ as pe,n as N,p as Se,e as ae,i as be,h as u,a3 as zn,a5 as tn,v as De,bq as Dt,bh as jt,aj as mn,br as Vn,t as Z,b7 as Ee,ap as Rn,bs as Wt,bt as un,_ as me,W as Vt,T as kn,a as K,m as G,b as ne,d as se,Y as Le,aY as Pn,q as Nn,b6 as Ht,U as Ut,an as Gt,u as je,ah as Hn,f as we,x as We,bu as qt,A as ue,aF as Ke,bv as Xt,a0 as Un,a4 as Yt,bw as Zt,aV as Jt,bx as Qt,aX as Mn,am as eo,aZ as no,aW as to,a$ as oo,X as fe,by as ro,c as On,z as Gn,bz as io,bA as qn,bB as lo,bC as ao,bD as so,a1 as uo,bE as co}from"./index-BRfpZIyJ.js";import{i as Xn,h as fo,j as Qe,k as Tn,c as ho,l as vo,d as cn,g as Yn,B as Zn,V as Jn,e as Qn,f as yn,u as po,r as go,p as et}from"./bindings-DNzsLWCw.js";import{N as bo,a as wn,b as mo,f as yo}from"./_plugin-vue_export-helper-Z0TH0GBI.js";function Te(e,n){let{target:t}=e;for(;t;){if(t.dataset&&t.dataset[n]!==void 0)return!0;t=t.parentElement}return!1}function wo(e={},n){const t=Et({ctrl:!1,command:!1,win:!1,shift:!1,tab:!1}),{keydown:o,keyup:r}=e,i=d=>{switch(d.key){case"Control":t.ctrl=!0;break;case"Meta":t.command=!0,t.win=!0;break;case"Shift":t.shift=!0;break;case"Tab":t.tab=!0;break}o!==void 0&&Object.keys(o).forEach(c=>{if(c!==d.key)return;const f=o[c];if(typeof f=="function")f(d);else{const{stop:h=!1,prevent:O=!1}=f;h&&d.stopPropagation(),O&&d.preventDefault(),f.handler(d)}})},l=d=>{switch(d.key){case"Control":t.ctrl=!1;break;case"Meta":t.command=!1,t.win=!1;break;case"Shift":t.shift=!1;break;case"Tab":t.tab=!1;break}r!==void 0&&Object.keys(r).forEach(c=>{if(c!==d.key)return;const f=r[c];if(typeof f=="function")f(d);else{const{stop:h=!1,prevent:O=!1}=f;h&&d.stopPropagation(),O&&d.preventDefault(),f.handler(d)}})},a=()=>{(n===void 0||n.value)&&(Ye("keydown",document,i),Ye("keyup",document,l)),n!==void 0&&ye(n,d=>{d?(Ye("keydown",document,i),Ye("keyup",document,l)):(Ze("keydown",document,i),Ze("keyup",document,l))})};return Kt()?(Lt(a),Cn(()=>{(n===void 0||n.value)&&(Ze("keydown",document,i),Ze("keyup",document,l))})):a(),$t(t)}function xo(e,n,t){const o=B(e.value);let r=null;return ye(e,i=>{r!==null&&window.clearTimeout(r),i===!0?t&&!t.value?o.value=!0:r=window.setTimeout(()=>{o.value=!0},n):o.value=!1}),o}function _n(e){return e&-e}class nt{constructor(n,t){this.l=n,this.min=t;const o=new Array(n+1);for(let r=0;r<n+1;++r)o[r]=0;this.ft=o}add(n,t){if(t===0)return;const{l:o,ft:r}=this;for(n+=1;n<=o;)r[n]+=t,n+=_n(n)}get(n){return this.sum(n+1)-this.sum(n)}sum(n){if(n===void 0&&(n=this.l),n<=0)return 0;const{ft:t,min:o,l:r}=this;if(n>r)throw new Error("[FinweckTree.sum]: `i` is larger than length.");let i=n*o;for(;n>0;)i+=t[n],n-=_n(n);return i}getBound(n){let t=0,o=this.l;for(;o>t;){const r=Math.floor((t+o)/2),i=this.sum(r);if(i>n){o=r;continue}else if(i<n){if(t===r)return this.sum(t+1)<=n?t+1:r;t=r}else return r}return t}}let Je;function So(){return typeof document>"u"?!1:(Je===void 0&&("matchMedia"in window?Je=window.matchMedia("(pointer:coarse)").matches:Je=!1),Je)}let fn;function An(){return typeof document>"u"?1:(fn===void 0&&(fn="chrome"in window?window.devicePixelRatio:1),fn)}const tt="VVirtualListXScroll";function Co({columnsRef:e,renderColRef:n,renderItemWithColsRef:t}){const o=B(0),r=B(0),i=N(()=>{const c=e.value;if(c.length===0)return null;const f=new nt(c.length,0);return c.forEach((h,O)=>{f.add(O,h.width)}),f}),l=pe(()=>{const c=i.value;return c!==null?Math.max(c.getBound(r.value)-1,0):0}),a=c=>{const f=i.value;return f!==null?f.sum(c):0},d=pe(()=>{const c=i.value;return c!==null?Math.min(c.getBound(r.value+o.value)+1,e.value.length-1):0});return Se(tt,{startIndexRef:l,endIndexRef:d,columnsRef:e,renderColRef:n,renderItemWithColsRef:t,getLeft:a}),{listWidthRef:o,scrollLeftRef:r}}const Bn=ae({name:"VirtualListRow",props:{index:{type:Number,required:!0},item:{type:Object,required:!0}},setup(){const{startIndexRef:e,endIndexRef:n,columnsRef:t,getLeft:o,renderColRef:r,renderItemWithColsRef:i}=be(tt);return{startIndex:e,endIndex:n,columns:t,renderCol:r,renderItemWithCols:i,getLeft:o}},render(){const{startIndex:e,endIndex:n,columns:t,renderCol:o,renderItemWithCols:r,getLeft:i,item:l}=this;if(r!=null)return r({itemIndex:this.index,startColIndex:e,endColIndex:n,allColumns:t,item:l,getLeft:i});if(o!=null){const a=[];for(let d=e;d<=n;++d){const c=t[d];a.push(o({column:c,left:i(d),item:l}))}return a}return null}}),Ro=Qe(".v-vl",{maxHeight:"inherit",height:"100%",overflow:"auto",minWidth:"1px"},[Qe("&:not(.v-vl--show-scrollbar)",{scrollbarWidth:"none"},[Qe("&::-webkit-scrollbar, &::-webkit-scrollbar-track-piece, &::-webkit-scrollbar-thumb",{width:0,height:0,display:"none"})])]),ko=ae({name:"VirtualList",inheritAttrs:!1,props:{showScrollbar:{type:Boolean,default:!0},columns:{type:Array,default:()=>[]},renderCol:Function,renderItemWithCols:Function,items:{type:Array,default:()=>[]},itemSize:{type:Number,required:!0},itemResizable:Boolean,itemsStyle:[String,Object],visibleItemsTag:{type:[String,Object],default:"div"},visibleItemsProps:Object,ignoreItemResize:Boolean,onScroll:Function,onWheel:Function,onResize:Function,defaultScrollKey:[Number,String],defaultScrollIndex:Number,keyField:{type:String,default:"key"},paddingTop:{type:[Number,String],default:0},paddingBottom:{type:[Number,String],default:0}},setup(e){const n=Vn();Ro.mount({id:"vueuc/virtual-list",head:!0,anchorMetaName:Xn,ssr:n}),De(()=>{const{defaultScrollIndex:v,defaultScrollKey:g}=e;v!=null?D({index:v}):g!=null&&D({key:g})});let t=!1,o=!1;Dt(()=>{if(t=!1,!o){o=!0;return}D({top:T.value,left:l.value})}),jt(()=>{t=!0,o||(o=!0)});const r=pe(()=>{if(e.renderCol==null&&e.renderItemWithCols==null||e.columns.length===0)return;let v=0;return e.columns.forEach(g=>{v+=g.width}),v}),i=N(()=>{const v=new Map,{keyField:g}=e;return e.items.forEach((k,P)=>{v.set(k[g],P)}),v}),{scrollLeftRef:l,listWidthRef:a}=Co({columnsRef:Z(e,"columns"),renderColRef:Z(e,"renderCol"),renderItemWithColsRef:Z(e,"renderItemWithCols")}),d=B(null),c=B(void 0),f=new Map,h=N(()=>{const{items:v,itemSize:g,keyField:k}=e,P=new nt(v.length,g);return v.forEach((y,z)=>{const E=y[k],Y=f.get(E);Y!==void 0&&P.add(z,Y)}),P}),O=B(0),T=B(0),m=pe(()=>Math.max(h.value.getBound(T.value-mn(e.paddingTop))-1,0)),$=N(()=>{const{value:v}=c;if(v===void 0)return[];const{items:g,itemSize:k}=e,P=m.value,y=Math.min(P+Math.ceil(v/k+1),g.length-1),z=[];for(let E=P;E<=y;++E)z.push(g[E]);return z}),D=(v,g)=>{if(typeof v=="number"){w(v,g,"auto");return}const{left:k,top:P,index:y,key:z,position:E,behavior:Y,debounce:ee=!0}=v;if(k!==void 0||P!==void 0)w(k,P,Y);else if(y!==void 0)_(y,Y,ee);else if(z!==void 0){const te=i.value.get(z);te!==void 0&&_(te,Y,ee)}else E==="bottom"?w(0,Number.MAX_SAFE_INTEGER,Y):E==="top"&&w(0,0,Y)};let F,I=null;function _(v,g,k){const{value:P}=h,y=P.sum(v)+mn(e.paddingTop);if(!k)d.value.scrollTo({left:0,top:y,behavior:g});else{F=v,I!==null&&window.clearTimeout(I),I=window.setTimeout(()=>{F=void 0,I=null},16);const{scrollTop:z,offsetHeight:E}=d.value;if(y>z){const Y=P.get(v);y+Y<=z+E||d.value.scrollTo({left:0,top:y+Y-E,behavior:g})}else d.value.scrollTo({left:0,top:y,behavior:g})}}function w(v,g,k){d.value.scrollTo({left:v,top:g,behavior:k})}function S(v,g){var k,P,y;if(t||e.ignoreItemResize||ie(g.target))return;const{value:z}=h,E=i.value.get(v),Y=z.get(E),ee=(y=(P=(k=g.borderBoxSize)===null||k===void 0?void 0:k[0])===null||P===void 0?void 0:P.blockSize)!==null&&y!==void 0?y:g.contentRect.height;if(ee===Y)return;ee-e.itemSize===0?f.delete(v):f.set(v,ee-e.itemSize);const de=ee-Y;if(de===0)return;z.add(E,de);const p=d.value;if(p!=null){if(F===void 0){const C=z.sum(E);p.scrollTop>C&&p.scrollBy(0,de)}else if(E<F)p.scrollBy(0,de);else if(E===F){const C=z.sum(E);ee+C>p.scrollTop+p.offsetHeight&&p.scrollBy(0,de)}Q()}O.value++}const A=!So();let H=!1;function q(v){var g;(g=e.onScroll)===null||g===void 0||g.call(e,v),(!A||!H)&&Q()}function J(v){var g;if((g=e.onWheel)===null||g===void 0||g.call(e,v),A){const k=d.value;if(k!=null){if(v.deltaX===0&&(k.scrollTop===0&&v.deltaY<=0||k.scrollTop+k.offsetHeight>=k.scrollHeight&&v.deltaY>=0))return;v.preventDefault(),k.scrollTop+=v.deltaY/An(),k.scrollLeft+=v.deltaX/An(),Q(),H=!0,fo(()=>{H=!1})}}}function V(v){if(t||ie(v.target))return;if(e.renderCol==null&&e.renderItemWithCols==null){if(v.contentRect.height===c.value)return}else if(v.contentRect.height===c.value&&v.contentRect.width===a.value)return;c.value=v.contentRect.height,a.value=v.contentRect.width;const{onResize:g}=e;g!==void 0&&g(v)}function Q(){const{value:v}=d;v!=null&&(T.value=v.scrollTop,l.value=v.scrollLeft)}function ie(v){let g=v;for(;g!==null;){if(g.style.display==="none")return!0;g=g.parentElement}return!1}return{listHeight:c,listStyle:{overflow:"auto"},keyToIndex:i,itemsStyle:N(()=>{const{itemResizable:v}=e,g=Ee(h.value.sum());return O.value,[e.itemsStyle,{boxSizing:"content-box",width:Ee(r.value),height:v?"":g,minHeight:v?g:"",paddingTop:Ee(e.paddingTop),paddingBottom:Ee(e.paddingBottom)}]}),visibleItemsStyle:N(()=>(O.value,{transform:`translateY(${Ee(h.value.sum(m.value))})`})),viewportItems:$,listElRef:d,itemsElRef:B(null),scrollTo:D,handleListResize:V,handleListScroll:q,handleListWheel:J,handleItemResize:S}},render(){const{itemResizable:e,keyField:n,keyToIndex:t,visibleItemsTag:o}=this;return u(zn,{onResize:this.handleListResize},{default:()=>{var r,i;return u("div",tn(this.$attrs,{class:["v-vl",this.showScrollbar&&"v-vl--show-scrollbar"],onScroll:this.handleListScroll,onWheel:this.handleListWheel,ref:"listElRef"}),[this.items.length!==0?u("div",{ref:"itemsElRef",class:"v-vl-items",style:this.itemsStyle},[u(o,Object.assign({class:"v-vl-visible-items",style:this.visibleItemsStyle},this.visibleItemsProps),{default:()=>{const{renderCol:l,renderItemWithCols:a}=this;return this.viewportItems.map(d=>{const c=d[n],f=t.get(c),h=l!=null?u(Bn,{index:f,item:d}):void 0,O=a!=null?u(Bn,{index:f,item:d}):void 0,T=this.$slots.default({item:d,renderedCols:h,renderedItemWithCols:O,index:f})[0];return e?u(zn,{key:c,onResize:m=>this.handleItemResize(c,m)},{default:()=>T}):(T.key=c,T)})}})]):(i=(r=this.$slots).empty)===null||i===void 0?void 0:i.call(r)])}})}}),xe="v-hidden",Po=Qe("[v-hidden]",{display:"none!important"}),$n=ae({name:"Overflow",props:{getCounter:Function,getTail:Function,updateCounter:Function,onUpdateCount:Function,onUpdateOverflow:Function},setup(e,{slots:n}){const t=B(null),o=B(null);function r(l){const{value:a}=t,{getCounter:d,getTail:c}=e;let f;if(d!==void 0?f=d():f=o.value,!a||!f)return;f.hasAttribute(xe)&&f.removeAttribute(xe);const{children:h}=a;if(l.showAllItemsBeforeCalculate)for(const _ of h)_.hasAttribute(xe)&&_.removeAttribute(xe);const O=a.offsetWidth,T=[],m=n.tail?c==null?void 0:c():null;let $=m?m.offsetWidth:0,D=!1;const F=a.children.length-(n.tail?1:0);for(let _=0;_<F-1;++_){if(_<0)continue;const w=h[_];if(D){w.hasAttribute(xe)||w.setAttribute(xe,"");continue}else w.hasAttribute(xe)&&w.removeAttribute(xe);const S=w.offsetWidth;if($+=S,T[_]=S,$>O){const{updateCounter:A}=e;for(let H=_;H>=0;--H){const q=F-1-H;A!==void 0?A(q):f.textContent=`${q}`;const J=f.offsetWidth;if($-=T[H],$+J<=O||H===0){D=!0,_=H-1,m&&(_===-1?(m.style.maxWidth=`${O-J}px`,m.style.boxSizing="border-box"):m.style.maxWidth="");const{onUpdateCount:V}=e;V&&V(q);break}}}}const{onUpdateOverflow:I}=e;D?I!==void 0&&I(!0):(I!==void 0&&I(!1),f.setAttribute(xe,""))}const i=Vn();return Po.mount({id:"vueuc/overflow",head:!0,anchorMetaName:Xn,ssr:i}),De(()=>r({showAllItemsBeforeCalculate:!1})),{selfRef:t,counterRef:o,sync:r}},render(){const{$slots:e}=this;return Rn(()=>this.sync({showAllItemsBeforeCalculate:!1})),u("div",{class:"v-overflow",ref:"selfRef"},[Wt(e,"default"),e.counter?e.counter():u("span",{style:{display:"inline-block"},ref:"counterRef"}),e.tail?e.tail():null])}});function ot(e,n){n&&(De(()=>{const{value:t}=e;t&&un.registerHandler(t,n)}),ye(e,(t,o)=>{o&&un.unregisterHandler(o)},{deep:!1}),Cn(()=>{const{value:t}=e;t&&un.unregisterHandler(t)}))}function En(e){switch(typeof e){case"string":return e||void 0;case"number":return String(e);default:return}}function Oo(e){return n=>{n?e.value=n.$el:e.value=null}}function hn(e){const n=e.filter(t=>t!==void 0);if(n.length!==0)return n.length===1?n[0]:t=>{e.forEach(o=>{o&&o(t)})}}const To=ae({name:"Checkmark",render(){return u("svg",{xmlns:"http://www.w3.org/2000/svg",viewBox:"0 0 16 16"},u("g",{fill:"none"},u("path",{d:"M14.046 3.486a.75.75 0 0 1-.032 1.06l-7.93 7.474a.85.85 0 0 1-1.188-.022l-2.68-2.72a.75.75 0 1 1 1.068-1.053l2.234 2.267l7.468-7.038a.75.75 0 0 1 1.06.032z",fill:"currentColor"})))}}),Fo=ae({name:"ChevronRight",render(){return u("svg",{viewBox:"0 0 16 16",fill:"none",xmlns:"http://www.w3.org/2000/svg"},u("path",{d:"M5.64645 3.14645C5.45118 3.34171 5.45118 3.65829 5.64645 3.85355L9.79289 8L5.64645 12.1464C5.45118 12.3417 5.45118 12.6583 5.64645 12.8536C5.84171 13.0488 6.15829 13.0488 6.35355 12.8536L10.8536 8.35355C11.0488 8.15829 11.0488 7.84171 10.8536 7.64645L6.35355 3.14645C6.15829 2.95118 5.84171 2.95118 5.64645 3.14645Z",fill:"currentColor"}))}}),Io=ae({props:{onFocus:Function,onBlur:Function},setup(e){return()=>u("div",{style:"width: 0; height: 0",tabindex:0,onFocus:e.onFocus,onBlur:e.onBlur})}});function Kn(e){return Array.isArray(e)?e:[e]}const xn={STOP:"STOP"};function rt(e,n){const t=n(e);e.children!==void 0&&t!==xn.STOP&&e.children.forEach(o=>rt(o,n))}function zo(e,n={}){const{preserveGroup:t=!1}=n,o=[],r=t?l=>{l.isLeaf||(o.push(l.key),i(l.children))}:l=>{l.isLeaf||(l.isGroup||o.push(l.key),i(l.children))};function i(l){l.forEach(r)}return i(e),o}function No(e,n){const{isLeaf:t}=e;return t!==void 0?t:!n(e)}function Mo(e){return e.children}function _o(e){return e.key}function Ao(){return!1}function Bo(e,n){const{isLeaf:t}=e;return!(t===!1&&!Array.isArray(n(e)))}function $o(e){return e.disabled===!0}function Eo(e,n){return e.isLeaf===!1&&!Array.isArray(n(e))}function vn(e){var n;return e==null?[]:Array.isArray(e)?e:(n=e.checkedKeys)!==null&&n!==void 0?n:[]}function pn(e){var n;return e==null||Array.isArray(e)?[]:(n=e.indeterminateKeys)!==null&&n!==void 0?n:[]}function Ko(e,n){const t=new Set(e);return n.forEach(o=>{t.has(o)||t.add(o)}),Array.from(t)}function Lo(e,n){const t=new Set(e);return n.forEach(o=>{t.has(o)&&t.delete(o)}),Array.from(t)}function Do(e){return(e==null?void 0:e.type)==="group"}function jo(e){const n=new Map;return e.forEach((t,o)=>{n.set(t.key,o)}),t=>{var o;return(o=n.get(t))!==null&&o!==void 0?o:null}}class Wo extends Error{constructor(){super(),this.message="SubtreeNotLoadedError: checking a subtree whose required nodes are not fully loaded."}}function Vo(e,n,t,o){return en(n.concat(e),t,o,!1)}function Ho(e,n){const t=new Set;return e.forEach(o=>{const r=n.treeNodeMap.get(o);if(r!==void 0){let i=r.parent;for(;i!==null&&!(i.disabled||t.has(i.key));)t.add(i.key),i=i.parent}}),t}function Uo(e,n,t,o){const r=en(n,t,o,!1),i=en(e,t,o,!0),l=Ho(e,t),a=[];return r.forEach(d=>{(i.has(d)||l.has(d))&&a.push(d)}),a.forEach(d=>r.delete(d)),r}function gn(e,n){const{checkedKeys:t,keysToCheck:o,keysToUncheck:r,indeterminateKeys:i,cascade:l,leafOnly:a,checkStrategy:d,allowNotLoaded:c}=e;if(!l)return o!==void 0?{checkedKeys:Ko(t,o),indeterminateKeys:Array.from(i)}:r!==void 0?{checkedKeys:Lo(t,r),indeterminateKeys:Array.from(i)}:{checkedKeys:Array.from(t),indeterminateKeys:Array.from(i)};const{levelTreeNodeMap:f}=n;let h;r!==void 0?h=Uo(r,t,n,c):o!==void 0?h=Vo(o,t,n,c):h=en(t,n,c,!1);const O=d==="parent",T=d==="child"||a,m=h,$=new Set,D=Math.max.apply(null,Array.from(f.keys()));for(let F=D;F>=0;F-=1){const I=F===0,_=f.get(F);for(const w of _){if(w.isLeaf)continue;const{key:S,shallowLoaded:A}=w;if(T&&A&&w.children.forEach(V=>{!V.disabled&&!V.isLeaf&&V.shallowLoaded&&m.has(V.key)&&m.delete(V.key)}),w.disabled||!A)continue;let H=!0,q=!1,J=!0;for(const V of w.children){const Q=V.key;if(!V.disabled){if(J&&(J=!1),m.has(Q))q=!0;else if($.has(Q)){q=!0,H=!1;break}else if(H=!1,q)break}}H&&!J?(O&&w.children.forEach(V=>{!V.disabled&&m.has(V.key)&&m.delete(V.key)}),m.add(S)):q&&$.add(S),I&&T&&m.has(S)&&m.delete(S)}}return{checkedKeys:Array.from(m),indeterminateKeys:Array.from($)}}function en(e,n,t,o){const{treeNodeMap:r,getChildren:i}=n,l=new Set,a=new Set(e);return e.forEach(d=>{const c=r.get(d);c!==void 0&&rt(c,f=>{if(f.disabled)return xn.STOP;const{key:h}=f;if(!l.has(h)&&(l.add(h),a.add(h),Eo(f.rawNode,i))){if(o)return xn.STOP;if(!t)throw new Wo}})}),a}function Go(e,{includeGroup:n=!1,includeSelf:t=!0},o){var r;const i=o.treeNodeMap;let l=e==null?null:(r=i.get(e))!==null&&r!==void 0?r:null;const a={keyPath:[],treeNodePath:[],treeNode:l};if(l!=null&&l.ignored)return a.treeNode=null,a;for(;l;)!l.ignored&&(n||!l.isGroup)&&a.treeNodePath.push(l),l=l.parent;return a.treeNodePath.reverse(),t||a.treeNodePath.pop(),a.keyPath=a.treeNodePath.map(d=>d.key),a}function qo(e){if(e.length===0)return null;const n=e[0];return n.isGroup||n.ignored||n.disabled?n.getNext():n}function Xo(e,n){const t=e.siblings,o=t.length,{index:r}=e;return n?t[(r+1)%o]:r===t.length-1?null:t[r+1]}function Ln(e,n,{loop:t=!1,includeDisabled:o=!1}={}){const r=n==="prev"?Yo:Xo,i={reverse:n==="prev"};let l=!1,a=null;function d(c){if(c!==null){if(c===e){if(!l)l=!0;else if(!e.disabled&&!e.isGroup){a=e;return}}else if((!c.disabled||o)&&!c.ignored&&!c.isGroup){a=c;return}if(c.isGroup){const f=Fn(c,i);f!==null?a=f:d(r(c,t))}else{const f=r(c,!1);if(f!==null)d(f);else{const h=Zo(c);h!=null&&h.isGroup?d(r(h,t)):t&&d(r(c,!0))}}}}return d(e),a}function Yo(e,n){const t=e.siblings,o=t.length,{index:r}=e;return n?t[(r-1+o)%o]:r===0?null:t[r-1]}function Zo(e){return e.parent}function Fn(e,n={}){const{reverse:t=!1}=n,{children:o}=e;if(o){const{length:r}=o,i=t?r-1:0,l=t?-1:r,a=t?-1:1;for(let d=i;d!==l;d+=a){const c=o[d];if(!c.disabled&&!c.ignored)if(c.isGroup){const f=Fn(c,n);if(f!==null)return f}else return c}}return null}const Jo={getChild(){return this.ignored?null:Fn(this)},getParent(){const{parent:e}=this;return e!=null&&e.isGroup?e.getParent():e},getNext(e={}){return Ln(this,"next",e)},getPrev(e={}){return Ln(this,"prev",e)}};function Qo(e,n){const t=n?new Set(n):void 0,o=[];function r(i){i.forEach(l=>{o.push(l),!(l.isLeaf||!l.children||l.ignored)&&(l.isGroup||t===void 0||t.has(l.key))&&r(l.children)})}return r(e),o}function er(e,n){const t=e.key;for(;n;){if(n.key===t)return!0;n=n.parent}return!1}function it(e,n,t,o,r,i=null,l=0){const a=[];return e.forEach((d,c)=>{var f;const h=Object.create(o);if(h.rawNode=d,h.siblings=a,h.level=l,h.index=c,h.isFirstChild=c===0,h.isLastChild=c+1===e.length,h.parent=i,!h.ignored){const O=r(d);Array.isArray(O)&&(h.children=it(O,n,t,o,r,h,l+1))}a.push(h),n.set(h.key,h),t.has(l)||t.set(l,[]),(f=t.get(l))===null||f===void 0||f.push(h)}),a}function lt(e,n={}){var t;const o=new Map,r=new Map,{getDisabled:i=$o,getIgnored:l=Ao,getIsGroup:a=Do,getKey:d=_o}=n,c=(t=n.getChildren)!==null&&t!==void 0?t:Mo,f=n.ignoreEmptyChildren?w=>{const S=c(w);return Array.isArray(S)?S.length?S:null:S}:c,h=Object.assign({get key(){return d(this.rawNode)},get disabled(){return i(this.rawNode)},get isGroup(){return a(this.rawNode)},get isLeaf(){return No(this.rawNode,f)},get shallowLoaded(){return Bo(this.rawNode,f)},get ignored(){return l(this.rawNode)},contains(w){return er(this,w)}},Jo),O=it(e,o,r,h,f);function T(w){if(w==null)return null;const S=o.get(w);return S&&!S.isGroup&&!S.ignored?S:null}function m(w){if(w==null)return null;const S=o.get(w);return S&&!S.ignored?S:null}function $(w,S){const A=m(w);return A?A.getPrev(S):null}function D(w,S){const A=m(w);return A?A.getNext(S):null}function F(w){const S=m(w);return S?S.getParent():null}function I(w){const S=m(w);return S?S.getChild():null}const _={treeNodes:O,treeNodeMap:o,levelTreeNodeMap:r,maxLevel:Math.max(...r.keys()),getChildren:f,getFlattenedNodes(w){return Qo(O,w)},getNode:T,getPrev:$,getNext:D,getParent:F,getChild:I,getFirstAvailableNode(){return qo(O)},getPath(w,S={}){return Go(w,S,_)},getCheckedKeys(w,S={}){const{cascade:A=!0,leafOnly:H=!1,checkStrategy:q="all",allowNotLoaded:J=!1}=S;return gn({checkedKeys:vn(w),indeterminateKeys:pn(w),cascade:A,leafOnly:H,checkStrategy:q,allowNotLoaded:J},_)},check(w,S,A={}){const{cascade:H=!0,leafOnly:q=!1,checkStrategy:J="all",allowNotLoaded:V=!1}=A;return gn({checkedKeys:vn(S),indeterminateKeys:pn(S),keysToCheck:w==null?[]:Kn(w),cascade:H,leafOnly:q,checkStrategy:J,allowNotLoaded:V},_)},uncheck(w,S,A={}){const{cascade:H=!0,leafOnly:q=!1,checkStrategy:J="all",allowNotLoaded:V=!1}=A;return gn({checkedKeys:vn(S),indeterminateKeys:pn(S),keysToUncheck:w==null?[]:Kn(w),cascade:H,leafOnly:q,checkStrategy:J,allowNotLoaded:V},_)},getNonLeafKeys(w={}){return zo(O,w)}};return _}const Dn=ae({name:"NBaseSelectGroupHeader",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(){const{renderLabelRef:e,renderOptionRef:n,labelFieldRef:t,nodePropsRef:o}=be(Tn);return{labelField:t,nodeProps:o,renderLabel:e,renderOption:n}},render(){const{clsPrefix:e,renderLabel:n,renderOption:t,nodeProps:o,tmNode:{rawNode:r}}=this,i=o==null?void 0:o(r),l=n?n(r,!1):me(r[this.labelField],r,!1),a=u("div",Object.assign({},i,{class:[`${e}-base-select-group-header`,i==null?void 0:i.class]}),l);return r.render?r.render({node:a,option:r}):t?t({node:a,option:r,selected:!1}):a}});function nr(e,n){return u(kn,{name:"fade-in-scale-up-transition"},{default:()=>e?u(Vt,{clsPrefix:n,class:`${n}-base-select-option__check`},{default:()=>u(To)}):null})}const jn=ae({name:"NBaseSelectOption",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(e){const{valueRef:n,pendingTmNodeRef:t,multipleRef:o,valueSetRef:r,renderLabelRef:i,renderOptionRef:l,labelFieldRef:a,valueFieldRef:d,showCheckmarkRef:c,nodePropsRef:f,handleOptionClick:h,handleOptionMouseEnter:O}=be(Tn),T=pe(()=>{const{value:F}=t;return F?e.tmNode.key===F.key:!1});function m(F){const{tmNode:I}=e;I.disabled||h(F,I)}function $(F){const{tmNode:I}=e;I.disabled||O(F,I)}function D(F){const{tmNode:I}=e,{value:_}=T;I.disabled||_||O(F,I)}return{multiple:o,isGrouped:pe(()=>{const{tmNode:F}=e,{parent:I}=F;return I&&I.rawNode.type==="group"}),showCheckmark:c,nodeProps:f,isPending:T,isSelected:pe(()=>{const{value:F}=n,{value:I}=o;if(F===null)return!1;const _=e.tmNode.rawNode[d.value];if(I){const{value:w}=r;return w.has(_)}else return F===_}),labelField:a,renderLabel:i,renderOption:l,handleMouseMove:D,handleMouseEnter:$,handleClick:m}},render(){const{clsPrefix:e,tmNode:{rawNode:n},isSelected:t,isPending:o,isGrouped:r,showCheckmark:i,nodeProps:l,renderOption:a,renderLabel:d,handleClick:c,handleMouseEnter:f,handleMouseMove:h}=this,O=nr(t,e),T=d?[d(n,t),i&&O]:[me(n[this.labelField],n,t),i&&O],m=l==null?void 0:l(n),$=u("div",Object.assign({},m,{class:[`${e}-base-select-option`,n.class,m==null?void 0:m.class,{[`${e}-base-select-option--disabled`]:n.disabled,[`${e}-base-select-option--selected`]:t,[`${e}-base-select-option--grouped`]:r,[`${e}-base-select-option--pending`]:o,[`${e}-base-select-option--show-checkmark`]:i}],style:[(m==null?void 0:m.style)||"",n.style||""],onClick:hn([c,m==null?void 0:m.onClick]),onMouseenter:hn([f,m==null?void 0:m.onMouseenter]),onMousemove:hn([h,m==null?void 0:m.onMousemove])}),u("div",{class:`${e}-base-select-option__content`},T));return n.render?n.render({node:$,option:n,selected:t}):a?a({node:$,option:n,selected:t}):$}}),tr=K("base-select-menu",`
 line-height: 1.5;
 outline: none;
 z-index: 0;
 position: relative;
 border-radius: var(--n-border-radius);
 transition:
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
 background-color: var(--n-color);
`,[K("scrollbar",`
 max-height: var(--n-height);
 `),K("virtual-list",`
 max-height: var(--n-height);
 `),K("base-select-option",`
 min-height: var(--n-option-height);
 font-size: var(--n-option-font-size);
 display: flex;
 align-items: center;
 `,[G("content",`
 z-index: 1;
 white-space: nowrap;
 text-overflow: ellipsis;
 overflow: hidden;
 `)]),K("base-select-group-header",`
 min-height: var(--n-option-height);
 font-size: .93em;
 display: flex;
 align-items: center;
 `),K("base-select-menu-option-wrapper",`
 position: relative;
 width: 100%;
 `),G("loading, empty",`
 display: flex;
 padding: 12px 32px;
 flex: 1;
 justify-content: center;
 `),G("loading",`
 color: var(--n-loading-color);
 font-size: var(--n-loading-size);
 `),G("header",`
 padding: 8px var(--n-option-padding-left);
 font-size: var(--n-option-font-size);
 transition: 
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 border-bottom: 1px solid var(--n-action-divider-color);
 color: var(--n-action-text-color);
 `),G("action",`
 padding: 8px var(--n-option-padding-left);
 font-size: var(--n-option-font-size);
 transition: 
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 border-top: 1px solid var(--n-action-divider-color);
 color: var(--n-action-text-color);
 `),K("base-select-group-header",`
 position: relative;
 cursor: default;
 padding: var(--n-option-padding);
 color: var(--n-group-header-text-color);
 `),K("base-select-option",`
 cursor: pointer;
 position: relative;
 padding: var(--n-option-padding);
 transition:
 color .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 box-sizing: border-box;
 color: var(--n-option-text-color);
 opacity: 1;
 `,[ne("show-checkmark",`
 padding-right: calc(var(--n-option-padding-right) + 20px);
 `),se("&::before",`
 content: "";
 position: absolute;
 left: 4px;
 right: 4px;
 top: 0;
 bottom: 0;
 border-radius: var(--n-border-radius);
 transition: background-color .3s var(--n-bezier);
 `),se("&:active",`
 color: var(--n-option-text-color-pressed);
 `),ne("grouped",`
 padding-left: calc(var(--n-option-padding-left) * 1.5);
 `),ne("pending",[se("&::before",`
 background-color: var(--n-option-color-pending);
 `)]),ne("selected",`
 color: var(--n-option-text-color-active);
 `,[se("&::before",`
 background-color: var(--n-option-color-active);
 `),ne("pending",[se("&::before",`
 background-color: var(--n-option-color-active-pending);
 `)])]),ne("disabled",`
 cursor: not-allowed;
 `,[Le("selected",`
 color: var(--n-option-text-color-disabled);
 `),ne("selected",`
 opacity: var(--n-option-opacity-disabled);
 `)]),G("check",`
 font-size: 16px;
 position: absolute;
 right: calc(var(--n-option-padding-right) - 4px);
 top: calc(50% - 7px);
 color: var(--n-option-check-color);
 transition: color .3s var(--n-bezier);
 `,[Pn({enterScale:"0.5"})])])]),or=ae({name:"InternalSelectMenu",props:Object.assign(Object.assign({},we.props),{clsPrefix:{type:String,required:!0},scrollable:{type:Boolean,default:!0},treeMate:{type:Object,required:!0},multiple:Boolean,size:{type:String,default:"medium"},value:{type:[String,Number,Array],default:null},autoPending:Boolean,virtualScroll:{type:Boolean,default:!0},show:{type:Boolean,default:!0},labelField:{type:String,default:"label"},valueField:{type:String,default:"value"},loading:Boolean,focusable:Boolean,renderLabel:Function,renderOption:Function,nodeProps:Function,showCheckmark:{type:Boolean,default:!0},onMousedown:Function,onScroll:Function,onFocus:Function,onBlur:Function,onKeyup:Function,onKeydown:Function,onTabOut:Function,onMouseenter:Function,onMouseleave:Function,onResize:Function,resetMenuOnOptionsChange:{type:Boolean,default:!0},inlineThemeDisabled:Boolean,scrollbarProps:Object,onToggle:Function}),setup(e){const{mergedClsPrefixRef:n,mergedRtlRef:t,mergedComponentPropsRef:o}=je(e),r=Hn("InternalSelectMenu",t,n),i=we("InternalSelectMenu","-internal-select-menu",tr,qt,e,Z(e,"clsPrefix")),l=B(null),a=B(null),d=B(null),c=N(()=>e.treeMate.getFlattenedNodes()),f=N(()=>jo(c.value)),h=B(null);function O(){const{treeMate:p}=e;let C=null;const{value:oe}=e;oe===null?C=p.getFirstAvailableNode():(e.multiple?C=p.getNode((oe||[])[(oe||[]).length-1]):C=p.getNode(oe),(!C||C.disabled)&&(C=p.getFirstAvailableNode())),P(C||null)}function T(){const{value:p}=h;p&&!e.treeMate.getNode(p.key)&&(h.value=null)}let m;ye(()=>e.show,p=>{p?m=ye(()=>e.treeMate,()=>{e.resetMenuOnOptionsChange?(e.autoPending?O():T(),Rn(y)):T()},{immediate:!0}):m==null||m()},{immediate:!0}),Cn(()=>{m==null||m()});const $=N(()=>mn(i.value.self[ue("optionHeight",e.size)])),D=N(()=>Ke(i.value.self[ue("padding",e.size)])),F=N(()=>e.multiple&&Array.isArray(e.value)?new Set(e.value):new Set),I=N(()=>{const p=c.value;return p&&p.length===0}),_=N(()=>{var p,C;return(C=(p=o==null?void 0:o.value)===null||p===void 0?void 0:p.Select)===null||C===void 0?void 0:C.renderEmpty});function w(p){const{onToggle:C}=e;C&&C(p)}function S(p){const{onScroll:C}=e;C&&C(p)}function A(p){var C;(C=d.value)===null||C===void 0||C.sync(),S(p)}function H(){var p;(p=d.value)===null||p===void 0||p.sync()}function q(){const{value:p}=h;return p||null}function J(p,C){C.disabled||P(C,!1)}function V(p,C){C.disabled||w(C)}function Q(p){var C;Te(p,"action")||(C=e.onKeyup)===null||C===void 0||C.call(e,p)}function ie(p){var C;Te(p,"action")||(C=e.onKeydown)===null||C===void 0||C.call(e,p)}function v(p){var C;(C=e.onMousedown)===null||C===void 0||C.call(e,p),!e.focusable&&p.preventDefault()}function g(){const{value:p}=h;p&&P(p.getNext({loop:!0}),!0)}function k(){const{value:p}=h;p&&P(p.getPrev({loop:!0}),!0)}function P(p,C=!1){h.value=p,C&&y()}function y(){var p,C;const oe=h.value;if(!oe)return;const he=f.value(oe.key);he!==null&&(e.virtualScroll?(p=a.value)===null||p===void 0||p.scrollTo({index:he}):(C=d.value)===null||C===void 0||C.scrollTo({index:he,elSize:$.value}))}function z(p){var C,oe;!((C=l.value)===null||C===void 0)&&C.contains(p.target)&&((oe=e.onFocus)===null||oe===void 0||oe.call(e,p))}function E(p){var C,oe;!((C=l.value)===null||C===void 0)&&C.contains(p.relatedTarget)||(oe=e.onBlur)===null||oe===void 0||oe.call(e,p)}Se(Tn,{handleOptionMouseEnter:J,handleOptionClick:V,valueSetRef:F,pendingTmNodeRef:h,nodePropsRef:Z(e,"nodeProps"),showCheckmarkRef:Z(e,"showCheckmark"),multipleRef:Z(e,"multiple"),valueRef:Z(e,"value"),renderLabelRef:Z(e,"renderLabel"),renderOptionRef:Z(e,"renderOption"),labelFieldRef:Z(e,"labelField"),valueFieldRef:Z(e,"valueField")}),Se(vo,l),De(()=>{const{value:p}=d;p&&p.sync()});const Y=N(()=>{const{size:p}=e,{common:{cubicBezierEaseInOut:C},self:{height:oe,borderRadius:he,color:Ce,groupHeaderTextColor:X,actionDividerColor:ce,optionTextColorPressed:Fe,optionTextColor:Re,optionTextColorDisabled:Ne,optionTextColorActive:Me,optionOpacityDisabled:_e,optionCheckColor:Pe,actionTextColor:Oe,optionColorPending:Ae,optionColorActive:Be,loadingColor:$e,loadingSize:Ie,optionColorActivePending:ze,[ue("optionFontSize",p)]:ve,[ue("optionHeight",p)]:b,[ue("optionPadding",p)]:R}}=i.value;return{"--n-height":oe,"--n-action-divider-color":ce,"--n-action-text-color":Oe,"--n-bezier":C,"--n-border-radius":he,"--n-color":Ce,"--n-option-font-size":ve,"--n-group-header-text-color":X,"--n-option-check-color":Pe,"--n-option-color-pending":Ae,"--n-option-color-active":Be,"--n-option-color-active-pending":ze,"--n-option-height":b,"--n-option-opacity-disabled":_e,"--n-option-text-color":Re,"--n-option-text-color-active":Me,"--n-option-text-color-disabled":Ne,"--n-option-text-color-pressed":Fe,"--n-option-padding":R,"--n-option-padding-left":Ke(R,"left"),"--n-option-padding-right":Ke(R,"right"),"--n-loading-color":$e,"--n-loading-size":Ie}}),{inlineThemeDisabled:ee}=e,te=ee?We("internal-select-menu",N(()=>e.size[0]),Y,e):void 0,de={selfRef:l,next:g,prev:k,getPendingTmNode:q};return ot(l,e.onResize),Object.assign({mergedTheme:i,mergedClsPrefix:n,rtlEnabled:r,virtualListRef:a,scrollbarRef:d,itemSize:$,padding:D,flattenedNodes:c,empty:I,mergedRenderEmpty:_,virtualListContainer(){const{value:p}=a;return p==null?void 0:p.listElRef},virtualListContent(){const{value:p}=a;return p==null?void 0:p.itemsElRef},doScroll:S,handleFocusin:z,handleFocusout:E,handleKeyUp:Q,handleKeyDown:ie,handleMouseDown:v,handleVirtualListResize:H,handleVirtualListScroll:A,cssVars:ee?void 0:Y,themeClass:te==null?void 0:te.themeClass,onRender:te==null?void 0:te.onRender},de)},render(){const{$slots:e,virtualScroll:n,clsPrefix:t,mergedTheme:o,themeClass:r,onRender:i}=this;return i==null||i(),u("div",{ref:"selfRef",tabindex:this.focusable?0:-1,class:[`${t}-base-select-menu`,`${t}-base-select-menu--${this.size}-size`,this.rtlEnabled&&`${t}-base-select-menu--rtl`,r,this.multiple&&`${t}-base-select-menu--multiple`],style:this.cssVars,onFocusin:this.handleFocusin,onFocusout:this.handleFocusout,onKeyup:this.handleKeyUp,onKeydown:this.handleKeyDown,onMousedown:this.handleMouseDown,onMouseenter:this.onMouseenter,onMouseleave:this.onMouseleave},Nn(e.header,l=>l&&u("div",{class:`${t}-base-select-menu__header`,"data-header":!0,key:"header"},l)),this.loading?u("div",{class:`${t}-base-select-menu__loading`},u(Ht,{clsPrefix:t,strokeWidth:20})):this.empty?u("div",{class:`${t}-base-select-menu__empty`,"data-empty":!0},Gt(e.empty,()=>{var l;return[((l=this.mergedRenderEmpty)===null||l===void 0?void 0:l.call(this))||u(ho,{theme:o.peers.Empty,themeOverrides:o.peerOverrides.Empty,size:this.size})]})):u(Ut,Object.assign({ref:"scrollbarRef",theme:o.peers.Scrollbar,themeOverrides:o.peerOverrides.Scrollbar,scrollable:this.scrollable,container:n?this.virtualListContainer:void 0,content:n?this.virtualListContent:void 0,onScroll:n?void 0:this.doScroll},this.scrollbarProps),{default:()=>n?u(ko,{ref:"virtualListRef",class:`${t}-virtual-list`,items:this.flattenedNodes,itemSize:this.itemSize,showScrollbar:!1,paddingTop:this.padding.top,paddingBottom:this.padding.bottom,onResize:this.handleVirtualListResize,onScroll:this.handleVirtualListScroll,itemResizable:!0},{default:({item:l})=>l.isGroup?u(Dn,{key:l.key,clsPrefix:t,tmNode:l}):l.ignored?null:u(jn,{clsPrefix:t,key:l.key,tmNode:l})}):u("div",{class:`${t}-base-select-menu-option-wrapper`,style:{paddingTop:this.padding.top,paddingBottom:this.padding.bottom}},this.flattenedNodes.map(l=>l.isGroup?u(Dn,{key:l.key,clsPrefix:t,tmNode:l}):u(jn,{clsPrefix:t,key:l.key,tmNode:l})))}),Nn(e.action,l=>l&&[u("div",{class:`${t}-base-select-menu__action`,"data-action":!0,key:"action"},l),u(Io,{onFocus:this.onTabOut,key:"focus-detector"})]))}}),rr=se([K("base-selection",`
 --n-padding-single: var(--n-padding-single-top) var(--n-padding-single-right) var(--n-padding-single-bottom) var(--n-padding-single-left);
 --n-padding-multiple: var(--n-padding-multiple-top) var(--n-padding-multiple-right) var(--n-padding-multiple-bottom) var(--n-padding-multiple-left);
 position: relative;
 z-index: auto;
 box-shadow: none;
 width: 100%;
 max-width: 100%;
 display: inline-block;
 vertical-align: bottom;
 border-radius: var(--n-border-radius);
 min-height: var(--n-height);
 line-height: 1.5;
 font-size: var(--n-font-size);
 `,[K("base-loading",`
 color: var(--n-loading-color);
 `),K("base-selection-tags","min-height: var(--n-height);"),G("border, state-border",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 pointer-events: none;
 border: var(--n-border);
 border-radius: inherit;
 transition:
 box-shadow .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `),G("state-border",`
 z-index: 1;
 border-color: #0000;
 `),K("base-suffix",`
 cursor: pointer;
 position: absolute;
 top: 50%;
 transform: translateY(-50%);
 right: 10px;
 `,[G("arrow",`
 font-size: var(--n-arrow-size);
 color: var(--n-arrow-color);
 transition: color .3s var(--n-bezier);
 `)]),K("base-selection-overlay",`
 display: flex;
 align-items: center;
 white-space: nowrap;
 pointer-events: none;
 position: absolute;
 top: 0;
 right: 0;
 bottom: 0;
 left: 0;
 padding: var(--n-padding-single);
 transition: color .3s var(--n-bezier);
 `,[G("wrapper",`
 flex-basis: 0;
 flex-grow: 1;
 overflow: hidden;
 text-overflow: ellipsis;
 `)]),K("base-selection-placeholder",`
 color: var(--n-placeholder-color);
 `,[G("inner",`
 max-width: 100%;
 overflow: hidden;
 `)]),K("base-selection-tags",`
 cursor: pointer;
 outline: none;
 box-sizing: border-box;
 position: relative;
 z-index: auto;
 display: flex;
 padding: var(--n-padding-multiple);
 flex-wrap: wrap;
 align-items: center;
 width: 100%;
 vertical-align: bottom;
 background-color: var(--n-color);
 border-radius: inherit;
 transition:
 color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 `),K("base-selection-label",`
 height: var(--n-height);
 display: inline-flex;
 width: 100%;
 vertical-align: bottom;
 cursor: pointer;
 outline: none;
 z-index: auto;
 box-sizing: border-box;
 position: relative;
 transition:
 color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 border-radius: inherit;
 background-color: var(--n-color);
 align-items: center;
 `,[K("base-selection-input",`
 font-size: inherit;
 line-height: inherit;
 outline: none;
 cursor: pointer;
 box-sizing: border-box;
 border:none;
 width: 100%;
 padding: var(--n-padding-single);
 background-color: #0000;
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 caret-color: var(--n-caret-color);
 `,[G("content",`
 text-overflow: ellipsis;
 overflow: hidden;
 white-space: nowrap; 
 `)]),G("render-label",`
 color: var(--n-text-color);
 `)]),Le("disabled",[se("&:hover",[G("state-border",`
 box-shadow: var(--n-box-shadow-hover);
 border: var(--n-border-hover);
 `)]),ne("focus",[G("state-border",`
 box-shadow: var(--n-box-shadow-focus);
 border: var(--n-border-focus);
 `)]),ne("active",[G("state-border",`
 box-shadow: var(--n-box-shadow-active);
 border: var(--n-border-active);
 `),K("base-selection-label","background-color: var(--n-color-active);"),K("base-selection-tags","background-color: var(--n-color-active);")])]),ne("disabled","cursor: not-allowed;",[G("arrow",`
 color: var(--n-arrow-color-disabled);
 `),K("base-selection-label",`
 cursor: not-allowed;
 background-color: var(--n-color-disabled);
 `,[K("base-selection-input",`
 cursor: not-allowed;
 color: var(--n-text-color-disabled);
 `),G("render-label",`
 color: var(--n-text-color-disabled);
 `)]),K("base-selection-tags",`
 cursor: not-allowed;
 background-color: var(--n-color-disabled);
 `),K("base-selection-placeholder",`
 cursor: not-allowed;
 color: var(--n-placeholder-color-disabled);
 `)]),K("base-selection-input-tag",`
 height: calc(var(--n-height) - 6px);
 line-height: calc(var(--n-height) - 6px);
 outline: none;
 display: none;
 position: relative;
 margin-bottom: 3px;
 max-width: 100%;
 vertical-align: bottom;
 `,[G("input",`
 font-size: inherit;
 font-family: inherit;
 min-width: 1px;
 padding: 0;
 background-color: #0000;
 outline: none;
 border: none;
 max-width: 100%;
 overflow: hidden;
 width: 1em;
 line-height: inherit;
 cursor: pointer;
 color: var(--n-text-color);
 caret-color: var(--n-caret-color);
 `),G("mirror",`
 position: absolute;
 left: 0;
 top: 0;
 white-space: pre;
 visibility: hidden;
 user-select: none;
 -webkit-user-select: none;
 opacity: 0;
 `)]),["warning","error"].map(e=>ne(`${e}-status`,[G("state-border",`border: var(--n-border-${e});`),Le("disabled",[se("&:hover",[G("state-border",`
 box-shadow: var(--n-box-shadow-hover-${e});
 border: var(--n-border-hover-${e});
 `)]),ne("active",[G("state-border",`
 box-shadow: var(--n-box-shadow-active-${e});
 border: var(--n-border-active-${e});
 `),K("base-selection-label",`background-color: var(--n-color-active-${e});`),K("base-selection-tags",`background-color: var(--n-color-active-${e});`)]),ne("focus",[G("state-border",`
 box-shadow: var(--n-box-shadow-focus-${e});
 border: var(--n-border-focus-${e});
 `)])])]))]),K("base-selection-popover",`
 margin-bottom: -3px;
 display: flex;
 flex-wrap: wrap;
 margin-right: -8px;
 `),K("base-selection-tag-wrapper",`
 max-width: 100%;
 display: inline-flex;
 padding: 0 7px 3px 0;
 `,[se("&:last-child","padding-right: 0;"),K("tag",`
 font-size: 14px;
 max-width: 100%;
 `,[G("content",`
 line-height: 1.25;
 text-overflow: ellipsis;
 overflow: hidden;
 `)])])]),ir=ae({name:"InternalSelection",props:Object.assign(Object.assign({},we.props),{clsPrefix:{type:String,required:!0},bordered:{type:Boolean,default:void 0},active:Boolean,pattern:{type:String,default:""},placeholder:String,selectedOption:{type:Object,default:null},selectedOptions:{type:Array,default:null},labelField:{type:String,default:"label"},valueField:{type:String,default:"value"},multiple:Boolean,filterable:Boolean,clearable:Boolean,disabled:Boolean,size:{type:String,default:"medium"},loading:Boolean,autofocus:Boolean,showArrow:{type:Boolean,default:!0},inputProps:Object,focused:Boolean,renderTag:Function,onKeydown:Function,onClick:Function,onBlur:Function,onFocus:Function,onDeleteOption:Function,maxTagCount:[String,Number],ellipsisTagPopoverProps:Object,onClear:Function,onPatternInput:Function,onPatternFocus:Function,onPatternBlur:Function,renderLabel:Function,status:String,inlineThemeDisabled:Boolean,ignoreComposition:{type:Boolean,default:!0},onResize:Function}),setup(e){const{mergedClsPrefixRef:n,mergedRtlRef:t}=je(e),o=Hn("InternalSelection",t,n),r=B(null),i=B(null),l=B(null),a=B(null),d=B(null),c=B(null),f=B(null),h=B(null),O=B(null),T=B(null),m=B(!1),$=B(!1),D=B(!1),F=we("InternalSelection","-internal-selection",rr,Zt,e,Z(e,"clsPrefix")),I=N(()=>e.clearable&&!e.disabled&&(D.value||e.active)),_=N(()=>e.selectedOption?e.renderTag?e.renderTag({option:e.selectedOption,handleClose:()=>{}}):e.renderLabel?e.renderLabel(e.selectedOption,!0):me(e.selectedOption[e.labelField],e.selectedOption,!0):e.placeholder),w=N(()=>{const b=e.selectedOption;if(b)return b[e.labelField]}),S=N(()=>e.multiple?!!(Array.isArray(e.selectedOptions)&&e.selectedOptions.length):e.selectedOption!==null);function A(){var b;const{value:R}=r;if(R){const{value:re}=i;re&&(re.style.width=`${R.offsetWidth}px`,e.maxTagCount!=="responsive"&&((b=O.value)===null||b===void 0||b.sync({showAllItemsBeforeCalculate:!1})))}}function H(){const{value:b}=T;b&&(b.style.display="none")}function q(){const{value:b}=T;b&&(b.style.display="inline-block")}ye(Z(e,"active"),b=>{b||H()}),ye(Z(e,"pattern"),()=>{e.multiple&&Rn(A)});function J(b){const{onFocus:R}=e;R&&R(b)}function V(b){const{onBlur:R}=e;R&&R(b)}function Q(b){const{onDeleteOption:R}=e;R&&R(b)}function ie(b){const{onClear:R}=e;R&&R(b)}function v(b){const{onPatternInput:R}=e;R&&R(b)}function g(b){var R;(!b.relatedTarget||!(!((R=l.value)===null||R===void 0)&&R.contains(b.relatedTarget)))&&J(b)}function k(b){var R;!((R=l.value)===null||R===void 0)&&R.contains(b.relatedTarget)||V(b)}function P(b){ie(b)}function y(){D.value=!0}function z(){D.value=!1}function E(b){!e.active||!e.filterable||b.target!==i.value&&b.preventDefault()}function Y(b){Q(b)}const ee=B(!1);function te(b){if(b.key==="Backspace"&&!ee.value&&!e.pattern.length){const{selectedOptions:R}=e;R!=null&&R.length&&Y(R[R.length-1])}}let de=null;function p(b){const{value:R}=r;if(R){const re=b.target.value;R.textContent=re,A()}e.ignoreComposition&&ee.value?de=b:v(b)}function C(){ee.value=!0}function oe(){ee.value=!1,e.ignoreComposition&&v(de),de=null}function he(b){var R;$.value=!0,(R=e.onPatternFocus)===null||R===void 0||R.call(e,b)}function Ce(b){var R;$.value=!1,(R=e.onPatternBlur)===null||R===void 0||R.call(e,b)}function X(){var b,R;if(e.filterable)$.value=!1,(b=c.value)===null||b===void 0||b.blur(),(R=i.value)===null||R===void 0||R.blur();else if(e.multiple){const{value:re}=a;re==null||re.blur()}else{const{value:re}=d;re==null||re.blur()}}function ce(){var b,R,re;e.filterable?($.value=!1,(b=c.value)===null||b===void 0||b.focus()):e.multiple?(R=a.value)===null||R===void 0||R.focus():(re=d.value)===null||re===void 0||re.focus()}function Fe(){const{value:b}=i;b&&(q(),b.focus())}function Re(){const{value:b}=i;b&&b.blur()}function Ne(b){const{value:R}=f;R&&R.setTextContent(`+${b}`)}function Me(){const{value:b}=h;return b}function _e(){return i.value}let Pe=null;function Oe(){Pe!==null&&window.clearTimeout(Pe)}function Ae(){e.active||(Oe(),Pe=window.setTimeout(()=>{S.value&&(m.value=!0)},100))}function Be(){Oe()}function $e(b){b||(Oe(),m.value=!1)}ye(S,b=>{b||(m.value=!1)}),De(()=>{Yt(()=>{const b=c.value;b&&(e.disabled?b.removeAttribute("tabindex"):b.tabIndex=$.value?-1:0)})}),ot(l,e.onResize);const{inlineThemeDisabled:Ie}=e,ze=N(()=>{const{size:b}=e,{common:{cubicBezierEaseInOut:R},self:{fontWeight:re,borderRadius:rn,color:ln,placeholderColor:an,textColor:Ve,paddingSingle:He,paddingMultiple:Ue,caretColor:sn,colorDisabled:dn,textColorDisabled:Ge,placeholderColorDisabled:ke,colorActive:s,boxShadowFocus:x,boxShadowActive:M,boxShadowHover:W,border:L,borderFocus:j,borderHover:U,borderActive:le,arrowColor:ge,arrowColorDisabled:ft,loadingColor:ht,colorActiveWarning:vt,boxShadowFocusWarning:pt,boxShadowActiveWarning:gt,boxShadowHoverWarning:bt,borderWarning:mt,borderFocusWarning:yt,borderHoverWarning:wt,borderActiveWarning:xt,colorActiveError:St,boxShadowFocusError:Ct,boxShadowActiveError:Rt,boxShadowHoverError:kt,borderError:Pt,borderFocusError:Ot,borderHoverError:Tt,borderActiveError:Ft,clearColor:It,clearColorHover:zt,clearColorPressed:Nt,clearSize:Mt,arrowSize:_t,[ue("height",b)]:At,[ue("fontSize",b)]:Bt}}=F.value,qe=Ke(He),Xe=Ke(Ue);return{"--n-bezier":R,"--n-border":L,"--n-border-active":le,"--n-border-focus":j,"--n-border-hover":U,"--n-border-radius":rn,"--n-box-shadow-active":M,"--n-box-shadow-focus":x,"--n-box-shadow-hover":W,"--n-caret-color":sn,"--n-color":ln,"--n-color-active":s,"--n-color-disabled":dn,"--n-font-size":Bt,"--n-height":At,"--n-padding-single-top":qe.top,"--n-padding-multiple-top":Xe.top,"--n-padding-single-right":qe.right,"--n-padding-multiple-right":Xe.right,"--n-padding-single-left":qe.left,"--n-padding-multiple-left":Xe.left,"--n-padding-single-bottom":qe.bottom,"--n-padding-multiple-bottom":Xe.bottom,"--n-placeholder-color":an,"--n-placeholder-color-disabled":ke,"--n-text-color":Ve,"--n-text-color-disabled":Ge,"--n-arrow-color":ge,"--n-arrow-color-disabled":ft,"--n-loading-color":ht,"--n-color-active-warning":vt,"--n-box-shadow-focus-warning":pt,"--n-box-shadow-active-warning":gt,"--n-box-shadow-hover-warning":bt,"--n-border-warning":mt,"--n-border-focus-warning":yt,"--n-border-hover-warning":wt,"--n-border-active-warning":xt,"--n-color-active-error":St,"--n-box-shadow-focus-error":Ct,"--n-box-shadow-active-error":Rt,"--n-box-shadow-hover-error":kt,"--n-border-error":Pt,"--n-border-focus-error":Ot,"--n-border-hover-error":Tt,"--n-border-active-error":Ft,"--n-clear-size":Mt,"--n-clear-color":It,"--n-clear-color-hover":zt,"--n-clear-color-pressed":Nt,"--n-arrow-size":_t,"--n-font-weight":re}}),ve=Ie?We("internal-selection",N(()=>e.size[0]),ze,e):void 0;return{mergedTheme:F,mergedClearable:I,mergedClsPrefix:n,rtlEnabled:o,patternInputFocused:$,filterablePlaceholder:_,label:w,selected:S,showTagsPanel:m,isComposing:ee,counterRef:f,counterWrapperRef:h,patternInputMirrorRef:r,patternInputRef:i,selfRef:l,multipleElRef:a,singleElRef:d,patternInputWrapperRef:c,overflowRef:O,inputTagElRef:T,handleMouseDown:E,handleFocusin:g,handleClear:P,handleMouseEnter:y,handleMouseLeave:z,handleDeleteOption:Y,handlePatternKeyDown:te,handlePatternInputInput:p,handlePatternInputBlur:Ce,handlePatternInputFocus:he,handleMouseEnterCounter:Ae,handleMouseLeaveCounter:Be,handleFocusout:k,handleCompositionEnd:oe,handleCompositionStart:C,onPopoverUpdateShow:$e,focus:ce,focusInput:Fe,blur:X,blurInput:Re,updateCounter:Ne,getCounter:Me,getTail:_e,renderLabel:e.renderLabel,cssVars:Ie?void 0:ze,themeClass:ve==null?void 0:ve.themeClass,onRender:ve==null?void 0:ve.onRender}},render(){const{status:e,multiple:n,size:t,disabled:o,filterable:r,maxTagCount:i,bordered:l,clsPrefix:a,ellipsisTagPopoverProps:d,onRender:c,renderTag:f,renderLabel:h}=this;c==null||c();const O=i==="responsive",T=typeof i=="number",m=O||T,$=u(Xt,null,{default:()=>u(bo,{clsPrefix:a,loading:this.loading,showArrow:this.showArrow,showClear:this.mergedClearable&&this.selected,onClear:this.handleClear},{default:()=>{var F,I;return(I=(F=this.$slots).arrow)===null||I===void 0?void 0:I.call(F)}})});let D;if(n){const{labelField:F}=this,I=v=>u("div",{class:`${a}-base-selection-tag-wrapper`,key:v.value},f?f({option:v,handleClose:()=>{this.handleDeleteOption(v)}}):u(cn,{size:t,closable:!v.disabled,disabled:o,onClose:()=>{this.handleDeleteOption(v)},internalCloseIsButtonTag:!1,internalCloseFocusable:!1},{default:()=>h?h(v,!0):me(v[F],v,!0)})),_=()=>(T?this.selectedOptions.slice(0,i):this.selectedOptions).map(I),w=r?u("div",{class:`${a}-base-selection-input-tag`,ref:"inputTagElRef",key:"__input-tag__"},u("input",Object.assign({},this.inputProps,{ref:"patternInputRef",tabindex:-1,disabled:o,value:this.pattern,autofocus:this.autofocus,class:`${a}-base-selection-input-tag__input`,onBlur:this.handlePatternInputBlur,onFocus:this.handlePatternInputFocus,onKeydown:this.handlePatternKeyDown,onInput:this.handlePatternInputInput,onCompositionstart:this.handleCompositionStart,onCompositionend:this.handleCompositionEnd})),u("span",{ref:"patternInputMirrorRef",class:`${a}-base-selection-input-tag__mirror`},this.pattern)):null,S=O?()=>u("div",{class:`${a}-base-selection-tag-wrapper`,ref:"counterWrapperRef"},u(cn,{size:t,ref:"counterRef",onMouseenter:this.handleMouseEnterCounter,onMouseleave:this.handleMouseLeaveCounter,disabled:o})):void 0;let A;if(T){const v=this.selectedOptions.length-i;v>0&&(A=u("div",{class:`${a}-base-selection-tag-wrapper`,key:"__counter__"},u(cn,{size:t,ref:"counterRef",onMouseenter:this.handleMouseEnterCounter,disabled:o},{default:()=>`+${v}`})))}const H=O?r?u($n,{ref:"overflowRef",updateCounter:this.updateCounter,getCounter:this.getCounter,getTail:this.getTail,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:_,counter:S,tail:()=>w}):u($n,{ref:"overflowRef",updateCounter:this.updateCounter,getCounter:this.getCounter,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:_,counter:S}):T&&A?_().concat(A):_(),q=m?()=>u("div",{class:`${a}-base-selection-popover`},O?_():this.selectedOptions.map(I)):void 0,J=m?Object.assign({show:this.showTagsPanel,trigger:"hover",overlap:!0,placement:"top",width:"trigger",onUpdateShow:this.onPopoverUpdateShow,theme:this.mergedTheme.peers.Popover,themeOverrides:this.mergedTheme.peerOverrides.Popover},d):null,Q=(this.selected?!1:this.active?!this.pattern&&!this.isComposing:!0)?u("div",{class:`${a}-base-selection-placeholder ${a}-base-selection-overlay`},u("div",{class:`${a}-base-selection-placeholder__inner`},this.placeholder)):null,ie=r?u("div",{ref:"patternInputWrapperRef",class:`${a}-base-selection-tags`},H,O?null:w,$):u("div",{ref:"multipleElRef",class:`${a}-base-selection-tags`,tabindex:o?void 0:0},H,$);D=u(Un,null,m?u(Yn,Object.assign({},J,{scrollable:!0,style:"max-height: calc(var(--v-target-height) * 6.6);"}),{trigger:()=>ie,default:q}):ie,Q)}else if(r){const F=this.pattern||this.isComposing,I=this.active?!F:!this.selected,_=this.active?!1:this.selected;D=u("div",{ref:"patternInputWrapperRef",class:`${a}-base-selection-label`,title:this.patternInputFocused?void 0:En(this.label)},u("input",Object.assign({},this.inputProps,{ref:"patternInputRef",class:`${a}-base-selection-input`,value:this.active?this.pattern:"",placeholder:"",readonly:o,disabled:o,tabindex:-1,autofocus:this.autofocus,onFocus:this.handlePatternInputFocus,onBlur:this.handlePatternInputBlur,onInput:this.handlePatternInputInput,onCompositionstart:this.handleCompositionStart,onCompositionend:this.handleCompositionEnd})),_?u("div",{class:`${a}-base-selection-label__render-label ${a}-base-selection-overlay`,key:"input"},u("div",{class:`${a}-base-selection-overlay__wrapper`},f?f({option:this.selectedOption,handleClose:()=>{}}):h?h(this.selectedOption,!0):me(this.label,this.selectedOption,!0))):null,I?u("div",{class:`${a}-base-selection-placeholder ${a}-base-selection-overlay`,key:"placeholder"},u("div",{class:`${a}-base-selection-overlay__wrapper`},this.filterablePlaceholder)):null,$)}else D=u("div",{ref:"singleElRef",class:`${a}-base-selection-label`,tabindex:this.disabled?void 0:0},this.label!==void 0?u("div",{class:`${a}-base-selection-input`,title:En(this.label),key:"input"},u("div",{class:`${a}-base-selection-input__content`},f?f({option:this.selectedOption,handleClose:()=>{}}):h?h(this.selectedOption,!0):me(this.label,this.selectedOption,!0))):u("div",{class:`${a}-base-selection-placeholder ${a}-base-selection-overlay`,key:"placeholder"},u("div",{class:`${a}-base-selection-placeholder__inner`},this.placeholder)),$);return u("div",{ref:"selfRef",class:[`${a}-base-selection`,this.rtlEnabled&&`${a}-base-selection--rtl`,this.themeClass,e&&`${a}-base-selection--${e}-status`,{[`${a}-base-selection--active`]:this.active,[`${a}-base-selection--selected`]:this.selected||this.active&&this.pattern,[`${a}-base-selection--disabled`]:this.disabled,[`${a}-base-selection--multiple`]:this.multiple,[`${a}-base-selection--focus`]:this.focused}],style:this.cssVars,onClick:this.onClick,onMouseenter:this.handleMouseEnter,onMouseleave:this.handleMouseLeave,onKeydown:this.onKeydown,onFocusin:this.handleFocusin,onFocusout:this.handleFocusout,onMousedown:this.handleMouseDown},D,l?u("div",{class:`${a}-base-selection__border`}):null,l?u("div",{class:`${a}-base-selection__state-border`}):null)}});function nn(e){return e.type==="group"}function at(e){return e.type==="ignored"}function bn(e,n){try{return!!(1+n.toString().toLowerCase().indexOf(e.trim().toLowerCase()))}catch{return!1}}function lr(e,n){return{getIsGroup:nn,getIgnored:at,getKey(o){return nn(o)?o.name||o.key||"key-required":o[e]},getChildren(o){return o[n]}}}function ar(e,n,t,o){if(!n)return e;function r(i){if(!Array.isArray(i))return[];const l=[];for(const a of i)if(nn(a)){const d=r(a[o]);d.length&&l.push(Object.assign({},a,{[o]:d}))}else{if(at(a))continue;n(t,a)&&l.push(a)}return l}return r(e)}function sr(e,n,t){const o=new Map;return e.forEach(r=>{nn(r)?r[t].forEach(i=>{o.set(i[n],i)}):o.set(r[n],r)}),o}const dr=se([K("select",`
 z-index: auto;
 outline: none;
 width: 100%;
 position: relative;
 font-weight: var(--n-font-weight);
 `),K("select-menu",`
 margin: 4px 0;
 box-shadow: var(--n-menu-box-shadow);
 `,[Pn({originalTransition:"background-color .3s var(--n-bezier), box-shadow .3s var(--n-bezier)"})])]),ur=Object.assign(Object.assign({},we.props),{to:yn.propTo,bordered:{type:Boolean,default:void 0},clearable:Boolean,clearCreatedOptionsOnClear:{type:Boolean,default:!0},clearFilterAfterSelect:{type:Boolean,default:!0},options:{type:Array,default:()=>[]},defaultValue:{type:[String,Number,Array],default:null},keyboard:{type:Boolean,default:!0},value:[String,Number,Array],placeholder:String,menuProps:Object,multiple:Boolean,size:String,menuSize:{type:String},filterable:Boolean,disabled:{type:Boolean,default:void 0},remote:Boolean,loading:Boolean,filter:Function,placement:{type:String,default:"bottom-start"},widthMode:{type:String,default:"trigger"},tag:Boolean,onCreate:Function,fallbackOption:{type:[Function,Boolean],default:void 0},show:{type:Boolean,default:void 0},showArrow:{type:Boolean,default:!0},maxTagCount:[Number,String],ellipsisTagPopoverProps:Object,consistentMenuWidth:{type:Boolean,default:!0},virtualScroll:{type:Boolean,default:!0},labelField:{type:String,default:"label"},valueField:{type:String,default:"value"},childrenField:{type:String,default:"children"},renderLabel:Function,renderOption:Function,renderTag:Function,"onUpdate:value":[Function,Array],inputProps:Object,nodeProps:Function,ignoreComposition:{type:Boolean,default:!0},showOnFocus:Boolean,onUpdateValue:[Function,Array],onBlur:[Function,Array],onClear:[Function,Array],onFocus:[Function,Array],onScroll:[Function,Array],onSearch:[Function,Array],onUpdateShow:[Function,Array],"onUpdate:show":[Function,Array],displayDirective:{type:String,default:"show"},resetMenuOnOptionsChange:{type:Boolean,default:!0},status:String,showCheckmark:{type:Boolean,default:!0},scrollbarProps:Object,onChange:[Function,Array],items:Array}),Pr=ae({name:"Select",props:ur,slots:Object,setup(e){const{mergedClsPrefixRef:n,mergedBorderedRef:t,namespaceRef:o,inlineThemeDisabled:r,mergedComponentPropsRef:i}=je(e),l=we("Select","-select",dr,ro,e,n),a=B(e.defaultValue),d=Z(e,"value"),c=wn(d,a),f=B(!1),h=B(""),O=po(e,["items","options"]),T=B([]),m=B([]),$=N(()=>m.value.concat(T.value).concat(O.value)),D=N(()=>{const{filter:s}=e;if(s)return s;const{labelField:x,valueField:M}=e;return(W,L)=>{if(!L)return!1;const j=L[x];if(typeof j=="string")return bn(W,j);const U=L[M];return typeof U=="string"?bn(W,U):typeof U=="number"?bn(W,String(U)):!1}}),F=N(()=>{if(e.remote)return O.value;{const{value:s}=$,{value:x}=h;return!x.length||!e.filterable?s:ar(s,D.value,x,e.childrenField)}}),I=N(()=>{const{valueField:s,childrenField:x}=e,M=lr(s,x);return lt(F.value,M)}),_=N(()=>sr($.value,e.valueField,e.childrenField)),w=B(!1),S=wn(Z(e,"show"),w),A=B(null),H=B(null),q=B(null),{localeRef:J}=mo("Select"),V=N(()=>{var s;return(s=e.placeholder)!==null&&s!==void 0?s:J.value.placeholder}),Q=[],ie=B(new Map),v=N(()=>{const{fallbackOption:s}=e;if(s===void 0){const{labelField:x,valueField:M}=e;return W=>({[x]:String(W),[M]:W})}return s===!1?!1:x=>Object.assign(s(x),{value:x})});function g(s){const x=e.remote,{value:M}=ie,{value:W}=_,{value:L}=v,j=[];return s.forEach(U=>{if(W.has(U))j.push(W.get(U));else if(x&&M.has(U))j.push(M.get(U));else if(L){const le=L(U);le&&j.push(le)}}),j}const k=N(()=>{if(e.multiple){const{value:s}=c;return Array.isArray(s)?g(s):[]}return null}),P=N(()=>{const{value:s}=c;return!e.multiple&&!Array.isArray(s)?s===null?null:g([s])[0]||null:null}),y=eo(e,{mergedSize:s=>{var x,M;const{size:W}=e;if(W)return W;const{mergedSize:L}=s||{};if(L!=null&&L.value)return L.value;const j=(M=(x=i==null?void 0:i.value)===null||x===void 0?void 0:x.Select)===null||M===void 0?void 0:M.size;return j||"medium"}}),{mergedSizeRef:z,mergedDisabledRef:E,mergedStatusRef:Y}=y;function ee(s,x){const{onChange:M,"onUpdate:value":W,onUpdateValue:L}=e,{nTriggerFormChange:j,nTriggerFormInput:U}=y;M&&fe(M,s,x),L&&fe(L,s,x),W&&fe(W,s,x),a.value=s,j(),U()}function te(s){const{onBlur:x}=e,{nTriggerFormBlur:M}=y;x&&fe(x,s),M()}function de(){const{onClear:s}=e;s&&fe(s)}function p(s){const{onFocus:x,showOnFocus:M}=e,{nTriggerFormFocus:W}=y;x&&fe(x,s),W(),M&&X()}function C(s){const{onSearch:x}=e;x&&fe(x,s)}function oe(s){const{onScroll:x}=e;x&&fe(x,s)}function he(){var s;const{remote:x,multiple:M}=e;if(x){const{value:W}=ie;if(M){const{valueField:L}=e;(s=k.value)===null||s===void 0||s.forEach(j=>{W.set(j[L],j)})}else{const L=P.value;L&&W.set(L[e.valueField],L)}}}function Ce(s){const{onUpdateShow:x,"onUpdate:show":M}=e;x&&fe(x,s),M&&fe(M,s),w.value=s}function X(){E.value||(Ce(!0),w.value=!0,e.filterable&&Ue())}function ce(){Ce(!1)}function Fe(){h.value="",m.value=Q}const Re=B(!1);function Ne(){e.filterable&&(Re.value=!0)}function Me(){e.filterable&&(Re.value=!1,S.value||Fe())}function _e(){E.value||(S.value?e.filterable?Ue():ce():X())}function Pe(s){var x,M;!((M=(x=q.value)===null||x===void 0?void 0:x.selfRef)===null||M===void 0)&&M.contains(s.relatedTarget)||(f.value=!1,te(s),ce())}function Oe(s){p(s),f.value=!0}function Ae(){f.value=!0}function Be(s){var x;!((x=A.value)===null||x===void 0)&&x.$el.contains(s.relatedTarget)||(f.value=!1,te(s),ce())}function $e(){var s;(s=A.value)===null||s===void 0||s.focus(),ce()}function Ie(s){var x;S.value&&(!((x=A.value)===null||x===void 0)&&x.$el.contains(to(s))||ce())}function ze(s){if(!Array.isArray(s))return[];if(v.value)return Array.from(s);{const{remote:x}=e,{value:M}=_;if(x){const{value:W}=ie;return s.filter(L=>M.has(L)||W.has(L))}else return s.filter(W=>M.has(W))}}function ve(s){b(s.rawNode)}function b(s){if(E.value)return;const{tag:x,remote:M,clearFilterAfterSelect:W,valueField:L}=e;if(x&&!M){const{value:j}=m,U=j[0]||null;if(U){const le=T.value;le.length?le.push(U):T.value=[U],m.value=Q}}if(M&&ie.value.set(s[L],s),e.multiple){const j=ze(c.value),U=j.findIndex(le=>le===s[L]);if(~U){if(j.splice(U,1),x&&!M){const le=R(s[L]);~le&&(T.value.splice(le,1),W&&(h.value=""))}}else j.push(s[L]),W&&(h.value="");ee(j,g(j))}else{if(x&&!M){const j=R(s[L]);~j?T.value=[T.value[j]]:T.value=Q}He(),ce(),ee(s[L],s)}}function R(s){return T.value.findIndex(M=>M[e.valueField]===s)}function re(s){S.value||X();const{value:x}=s.target;h.value=x;const{tag:M,remote:W}=e;if(C(x),M&&!W){if(!x){m.value=Q;return}const{onCreate:L}=e,j=L?L(x):{[e.labelField]:x,[e.valueField]:x},{valueField:U,labelField:le}=e;O.value.some(ge=>ge[U]===j[U]||ge[le]===j[le])||T.value.some(ge=>ge[U]===j[U]||ge[le]===j[le])?m.value=Q:m.value=[j]}}function rn(s){s.stopPropagation();const{multiple:x,tag:M,remote:W,clearCreatedOptionsOnClear:L}=e;!x&&e.filterable&&ce(),M&&!W&&L&&(T.value=Q),de(),x?ee([],[]):ee(null,null)}function ln(s){!Te(s,"action")&&!Te(s,"empty")&&!Te(s,"header")&&s.preventDefault()}function an(s){oe(s)}function Ve(s){var x,M,W,L,j;if(!e.keyboard){s.preventDefault();return}switch(s.key){case" ":if(e.filterable)break;s.preventDefault();case"Enter":if(!(!((x=A.value)===null||x===void 0)&&x.isComposing)){if(S.value){const U=(M=q.value)===null||M===void 0?void 0:M.getPendingTmNode();U?ve(U):e.filterable||(ce(),He())}else if(X(),e.tag&&Re.value){const U=m.value[0];if(U){const le=U[e.valueField],{value:ge}=c;e.multiple&&Array.isArray(ge)&&ge.includes(le)||b(U)}}}s.preventDefault();break;case"ArrowUp":if(s.preventDefault(),e.loading)return;S.value&&((W=q.value)===null||W===void 0||W.prev());break;case"ArrowDown":if(s.preventDefault(),e.loading)return;S.value?(L=q.value)===null||L===void 0||L.next():X();break;case"Escape":S.value&&(oo(s),ce()),(j=A.value)===null||j===void 0||j.focus();break}}function He(){var s;(s=A.value)===null||s===void 0||s.focus()}function Ue(){var s;(s=A.value)===null||s===void 0||s.focusInput()}function sn(){var s;S.value&&((s=H.value)===null||s===void 0||s.syncPosition())}he(),ye(Z(e,"options"),he);const dn={focus:()=>{var s;(s=A.value)===null||s===void 0||s.focus()},focusInput:()=>{var s;(s=A.value)===null||s===void 0||s.focusInput()},blur:()=>{var s;(s=A.value)===null||s===void 0||s.blur()},blurInput:()=>{var s;(s=A.value)===null||s===void 0||s.blurInput()}},Ge=N(()=>{const{self:{menuBoxShadow:s}}=l.value;return{"--n-menu-box-shadow":s}}),ke=r?We("select",void 0,Ge,e):void 0;return Object.assign(Object.assign({},dn),{mergedStatus:Y,mergedClsPrefix:n,mergedBordered:t,namespace:o,treeMate:I,isMounted:no(),triggerRef:A,menuRef:q,pattern:h,uncontrolledShow:w,mergedShow:S,adjustedTo:yn(e),uncontrolledValue:a,mergedValue:c,followerRef:H,localizedPlaceholder:V,selectedOption:P,selectedOptions:k,mergedSize:z,mergedDisabled:E,focused:f,activeWithoutMenuOpen:Re,inlineThemeDisabled:r,onTriggerInputFocus:Ne,onTriggerInputBlur:Me,handleTriggerOrMenuResize:sn,handleMenuFocus:Ae,handleMenuBlur:Be,handleMenuTabOut:$e,handleTriggerClick:_e,handleToggle:ve,handleDeleteOption:b,handlePatternInput:re,handleClear:rn,handleTriggerBlur:Pe,handleTriggerFocus:Oe,handleKeydown:Ve,handleMenuAfterLeave:Fe,handleMenuClickOutside:Ie,handleMenuScroll:an,handleMenuKeydown:Ve,handleMenuMousedown:ln,mergedTheme:l,cssVars:r?void 0:Ge,themeClass:ke==null?void 0:ke.themeClass,onRender:ke==null?void 0:ke.onRender})},render(){return u("div",{class:`${this.mergedClsPrefix}-select`},u(Zn,null,{default:()=>[u(Jn,null,{default:()=>u(ir,{ref:"triggerRef",inlineThemeDisabled:this.inlineThemeDisabled,status:this.mergedStatus,inputProps:this.inputProps,clsPrefix:this.mergedClsPrefix,showArrow:this.showArrow,maxTagCount:this.maxTagCount,ellipsisTagPopoverProps:this.ellipsisTagPopoverProps,bordered:this.mergedBordered,active:this.activeWithoutMenuOpen||this.mergedShow,pattern:this.pattern,placeholder:this.localizedPlaceholder,selectedOption:this.selectedOption,selectedOptions:this.selectedOptions,multiple:this.multiple,renderTag:this.renderTag,renderLabel:this.renderLabel,filterable:this.filterable,clearable:this.clearable,disabled:this.mergedDisabled,size:this.mergedSize,theme:this.mergedTheme.peers.InternalSelection,labelField:this.labelField,valueField:this.valueField,themeOverrides:this.mergedTheme.peerOverrides.InternalSelection,loading:this.loading,focused:this.focused,onClick:this.handleTriggerClick,onDeleteOption:this.handleDeleteOption,onPatternInput:this.handlePatternInput,onClear:this.handleClear,onBlur:this.handleTriggerBlur,onFocus:this.handleTriggerFocus,onKeydown:this.handleKeydown,onPatternBlur:this.onTriggerInputBlur,onPatternFocus:this.onTriggerInputFocus,onResize:this.handleTriggerOrMenuResize,ignoreComposition:this.ignoreComposition},{arrow:()=>{var e,n;return[(n=(e=this.$slots).arrow)===null||n===void 0?void 0:n.call(e)]}})}),u(Qn,{ref:"followerRef",show:this.mergedShow,to:this.adjustedTo,teleportDisabled:this.adjustedTo===yn.tdkey,containerClass:this.namespace,width:this.consistentMenuWidth?"target":void 0,minWidth:"target",placement:this.placement},{default:()=>u(kn,{name:"fade-in-scale-up-transition",appear:this.isMounted,onAfterLeave:this.handleMenuAfterLeave},{default:()=>{var e,n,t;return this.mergedShow||this.displayDirective==="show"?((e=this.onRender)===null||e===void 0||e.call(this),Jt(u(or,Object.assign({},this.menuProps,{ref:"menuRef",onResize:this.handleTriggerOrMenuResize,inlineThemeDisabled:this.inlineThemeDisabled,virtualScroll:this.consistentMenuWidth&&this.virtualScroll,class:[`${this.mergedClsPrefix}-select-menu`,this.themeClass,(n=this.menuProps)===null||n===void 0?void 0:n.class],clsPrefix:this.mergedClsPrefix,focusable:!0,labelField:this.labelField,valueField:this.valueField,autoPending:!0,nodeProps:this.nodeProps,theme:this.mergedTheme.peers.InternalSelectMenu,themeOverrides:this.mergedTheme.peerOverrides.InternalSelectMenu,treeMate:this.treeMate,multiple:this.multiple,size:this.menuSize,renderOption:this.renderOption,renderLabel:this.renderLabel,value:this.mergedValue,style:[(t=this.menuProps)===null||t===void 0?void 0:t.style,this.cssVars],onToggle:this.handleToggle,onScroll:this.handleMenuScroll,onFocus:this.handleMenuFocus,onBlur:this.handleMenuBlur,onKeydown:this.handleMenuKeydown,onTabOut:this.handleMenuTabOut,onMousedown:this.handleMenuMousedown,show:this.mergedShow,showCheckmark:this.showCheckmark,resetMenuOnOptionsChange:this.resetMenuOnOptionsChange,scrollbarProps:this.scrollbarProps}),{empty:()=>{var o,r;return[(r=(o=this.$slots).empty)===null||r===void 0?void 0:r.call(o)]},header:()=>{var o,r;return[(r=(o=this.$slots).header)===null||r===void 0?void 0:r.call(o)]},action:()=>{var o,r;return[(r=(o=this.$slots).action)===null||r===void 0?void 0:r.call(o)]}}),this.displayDirective==="show"?[[Qt,this.mergedShow],[Mn,this.handleMenuClickOutside,void 0,{capture:!0}]]:[[Mn,this.handleMenuClickOutside,void 0,{capture:!0}]])):null}})})]}))}}),In=On("n-dropdown-menu"),on=On("n-dropdown"),Wn=On("n-dropdown-option"),st=ae({name:"DropdownDivider",props:{clsPrefix:{type:String,required:!0}},render(){return u("div",{class:`${this.clsPrefix}-dropdown-divider`})}}),cr=ae({name:"DropdownGroupHeader",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(){const{showIconRef:e,hasSubmenuRef:n}=be(In),{renderLabelRef:t,labelFieldRef:o,nodePropsRef:r,renderOptionRef:i}=be(on);return{labelField:o,showIcon:e,hasSubmenu:n,renderLabel:t,nodeProps:r,renderOption:i}},render(){var e;const{clsPrefix:n,hasSubmenu:t,showIcon:o,nodeProps:r,renderLabel:i,renderOption:l}=this,{rawNode:a}=this.tmNode,d=u("div",Object.assign({class:`${n}-dropdown-option`},r==null?void 0:r(a)),u("div",{class:`${n}-dropdown-option-body ${n}-dropdown-option-body--group`},u("div",{"data-dropdown-option":!0,class:[`${n}-dropdown-option-body__prefix`,o&&`${n}-dropdown-option-body__prefix--show-icon`]},me(a.icon)),u("div",{class:`${n}-dropdown-option-body__label`,"data-dropdown-option":!0},i?i(a):me((e=a.title)!==null&&e!==void 0?e:a[this.labelField])),u("div",{class:[`${n}-dropdown-option-body__suffix`,t&&`${n}-dropdown-option-body__suffix--has-submenu`],"data-dropdown-option":!0})));return l?l({node:d,option:a}):d}}),fr=K("icon",`
 height: 1em;
 width: 1em;
 line-height: 1em;
 text-align: center;
 display: inline-block;
 position: relative;
 fill: currentColor;
`,[ne("color-transition",{transition:"color .3s var(--n-bezier)"}),ne("depth",{color:"var(--n-color)"},[se("svg",{opacity:"var(--n-opacity)",transition:"opacity .3s var(--n-bezier)"})]),se("svg",{height:"1em",width:"1em"})]),hr=Object.assign(Object.assign({},we.props),{depth:[String,Number],size:[Number,String],color:String,component:[Object,Function]}),vr=ae({_n_icon__:!0,name:"Icon",inheritAttrs:!1,props:hr,setup(e){const{mergedClsPrefixRef:n,inlineThemeDisabled:t}=je(e),o=we("Icon","-icon",fr,io,e,n),r=N(()=>{const{depth:l}=e,{common:{cubicBezierEaseInOut:a},self:d}=o.value;if(l!==void 0){const{color:c,[`opacity${l}Depth`]:f}=d;return{"--n-bezier":a,"--n-color":c,"--n-opacity":f}}return{"--n-bezier":a,"--n-color":"","--n-opacity":""}}),i=t?We("icon",N(()=>`${e.depth||"d"}`),r,e):void 0;return{mergedClsPrefix:n,mergedStyle:N(()=>{const{size:l,color:a}=e;return{fontSize:yo(l),color:a}}),cssVars:t?void 0:r,themeClass:i==null?void 0:i.themeClass,onRender:i==null?void 0:i.onRender}},render(){var e;const{$parent:n,depth:t,mergedClsPrefix:o,component:r,onRender:i,themeClass:l}=this;return!((e=n==null?void 0:n.$options)===null||e===void 0)&&e._n_icon__&&Gn("icon","don't wrap `n-icon` inside `n-icon`"),i==null||i(),u("i",tn(this.$attrs,{role:"img",class:[`${o}-icon`,l,{[`${o}-icon--depth`]:t,[`${o}-icon--color-transition`]:t!==void 0}],style:[this.cssVars,this.mergedStyle]}),r?u(r):this.$slots)}});function Sn(e,n){return e.type==="submenu"||e.type===void 0&&e[n]!==void 0}function pr(e){return e.type==="group"}function dt(e){return e.type==="divider"}function gr(e){return e.type==="render"}const ut=ae({name:"DropdownOption",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0},parentKey:{type:[String,Number],default:null},placement:{type:String,default:"right-start"},props:Object,scrollable:Boolean},setup(e){const n=be(on),{hoverKeyRef:t,keyboardKeyRef:o,lastToggledSubmenuKeyRef:r,pendingKeyPathRef:i,activeKeyPathRef:l,animatedRef:a,mergedShowRef:d,renderLabelRef:c,renderIconRef:f,labelFieldRef:h,childrenFieldRef:O,renderOptionRef:T,nodePropsRef:m,menuPropsRef:$}=n,D=be(Wn,null),F=be(In),I=be(qn),_=N(()=>e.tmNode.rawNode),w=N(()=>{const{value:y}=O;return Sn(e.tmNode.rawNode,y)}),S=N(()=>{const{disabled:y}=e.tmNode;return y}),A=N(()=>{if(!w.value)return!1;const{key:y,disabled:z}=e.tmNode;if(z)return!1;const{value:E}=t,{value:Y}=o,{value:ee}=r,{value:te}=i;return E!==null?te.includes(y):Y!==null?te.includes(y)&&te[te.length-1]!==y:ee!==null?te.includes(y):!1}),H=N(()=>o.value===null&&!a.value),q=xo(A,300,H),J=N(()=>!!(D!=null&&D.enteringSubmenuRef.value)),V=B(!1);Se(Wn,{enteringSubmenuRef:V});function Q(){V.value=!0}function ie(){V.value=!1}function v(){const{parentKey:y,tmNode:z}=e;z.disabled||d.value&&(r.value=y,o.value=null,t.value=z.key)}function g(){const{tmNode:y}=e;y.disabled||d.value&&t.value!==y.key&&v()}function k(y){if(e.tmNode.disabled||!d.value)return;const{relatedTarget:z}=y;z&&!Te({target:z},"dropdownOption")&&!Te({target:z},"scrollbarRail")&&(t.value=null)}function P(){const{value:y}=w,{tmNode:z}=e;d.value&&!y&&!z.disabled&&(n.doSelect(z.key,z.rawNode),n.doUpdateShow(!1))}return{labelField:h,renderLabel:c,renderIcon:f,siblingHasIcon:F.showIconRef,siblingHasSubmenu:F.hasSubmenuRef,menuProps:$,popoverBody:I,animated:a,mergedShowSubmenu:N(()=>q.value&&!J.value),rawNode:_,hasSubmenu:w,pending:pe(()=>{const{value:y}=i,{key:z}=e.tmNode;return y.includes(z)}),childActive:pe(()=>{const{value:y}=l,{key:z}=e.tmNode,E=y.findIndex(Y=>z===Y);return E===-1?!1:E<y.length-1}),active:pe(()=>{const{value:y}=l,{key:z}=e.tmNode,E=y.findIndex(Y=>z===Y);return E===-1?!1:E===y.length-1}),mergedDisabled:S,renderOption:T,nodeProps:m,handleClick:P,handleMouseMove:g,handleMouseEnter:v,handleMouseLeave:k,handleSubmenuBeforeEnter:Q,handleSubmenuAfterEnter:ie}},render(){var e,n;const{animated:t,rawNode:o,mergedShowSubmenu:r,clsPrefix:i,siblingHasIcon:l,siblingHasSubmenu:a,renderLabel:d,renderIcon:c,renderOption:f,nodeProps:h,props:O,scrollable:T}=this;let m=null;if(r){const I=(e=this.menuProps)===null||e===void 0?void 0:e.call(this,o,o.children);m=u(ct,Object.assign({},I,{clsPrefix:i,scrollable:this.scrollable,tmNodes:this.tmNode.children,parentKey:this.tmNode.key}))}const $={class:[`${i}-dropdown-option-body`,this.pending&&`${i}-dropdown-option-body--pending`,this.active&&`${i}-dropdown-option-body--active`,this.childActive&&`${i}-dropdown-option-body--child-active`,this.mergedDisabled&&`${i}-dropdown-option-body--disabled`],onMousemove:this.handleMouseMove,onMouseenter:this.handleMouseEnter,onMouseleave:this.handleMouseLeave,onClick:this.handleClick},D=h==null?void 0:h(o),F=u("div",Object.assign({class:[`${i}-dropdown-option`,D==null?void 0:D.class],"data-dropdown-option":!0},D),u("div",tn($,O),[u("div",{class:[`${i}-dropdown-option-body__prefix`,l&&`${i}-dropdown-option-body__prefix--show-icon`]},[c?c(o):me(o.icon)]),u("div",{"data-dropdown-option":!0,class:`${i}-dropdown-option-body__label`},d?d(o):me((n=o[this.labelField])!==null&&n!==void 0?n:o.title)),u("div",{"data-dropdown-option":!0,class:[`${i}-dropdown-option-body__suffix`,a&&`${i}-dropdown-option-body__suffix--has-submenu`]},this.hasSubmenu?u(vr,null,{default:()=>u(Fo,null)}):null)]),this.hasSubmenu?u(Zn,null,{default:()=>[u(Jn,null,{default:()=>u("div",{class:`${i}-dropdown-offset-container`},u(Qn,{show:this.mergedShowSubmenu,placement:this.placement,to:T&&this.popoverBody||void 0,teleportDisabled:!T},{default:()=>u("div",{class:`${i}-dropdown-menu-wrapper`},t?u(kn,{onBeforeEnter:this.handleSubmenuBeforeEnter,onAfterEnter:this.handleSubmenuAfterEnter,name:"fade-in-scale-up-transition",appear:!0},{default:()=>m}):m)}))})]}):null);return f?f({node:F,option:o}):F}}),br=ae({name:"NDropdownGroup",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0},parentKey:{type:[String,Number],default:null}},render(){const{tmNode:e,parentKey:n,clsPrefix:t}=this,{children:o}=e;return u(Un,null,u(cr,{clsPrefix:t,tmNode:e,key:e.key}),o==null?void 0:o.map(r=>{const{rawNode:i}=r;return i.show===!1?null:dt(i)?u(st,{clsPrefix:t,key:r.key}):r.isGroup?(Gn("dropdown","`group` node is not allowed to be put in `group` node."),null):u(ut,{clsPrefix:t,tmNode:r,parentKey:n,key:r.key})}))}}),mr=ae({name:"DropdownRenderOption",props:{tmNode:{type:Object,required:!0}},render(){const{rawNode:{render:e,props:n}}=this.tmNode;return u("div",n,[e==null?void 0:e()])}}),ct=ae({name:"DropdownMenu",props:{scrollable:Boolean,showArrow:Boolean,arrowStyle:[String,Object],clsPrefix:{type:String,required:!0},tmNodes:{type:Array,default:()=>[]},parentKey:{type:[String,Number],default:null}},setup(e){const{renderIconRef:n,childrenFieldRef:t}=be(on);Se(In,{showIconRef:N(()=>{const r=n.value;return e.tmNodes.some(i=>{var l;if(i.isGroup)return(l=i.children)===null||l===void 0?void 0:l.some(({rawNode:d})=>r?r(d):d.icon);const{rawNode:a}=i;return r?r(a):a.icon})}),hasSubmenuRef:N(()=>{const{value:r}=t;return e.tmNodes.some(i=>{var l;if(i.isGroup)return(l=i.children)===null||l===void 0?void 0:l.some(({rawNode:d})=>Sn(d,r));const{rawNode:a}=i;return Sn(a,r)})})});const o=B(null);return Se(ao,null),Se(so,null),Se(qn,o),{bodyRef:o}},render(){const{parentKey:e,clsPrefix:n,scrollable:t}=this,o=this.tmNodes.map(r=>{const{rawNode:i}=r;return i.show===!1?null:gr(i)?u(mr,{tmNode:r,key:r.key}):dt(i)?u(st,{clsPrefix:n,key:r.key}):pr(i)?u(br,{clsPrefix:n,tmNode:r,parentKey:e,key:r.key}):u(ut,{clsPrefix:n,tmNode:r,parentKey:e,key:r.key,props:i.props,scrollable:t})});return u("div",{class:[`${n}-dropdown-menu`,t&&`${n}-dropdown-menu--scrollable`],ref:"bodyRef"},t?u(lo,{contentClass:`${n}-dropdown-menu__content`},{default:()=>o}):o,this.showArrow?go({clsPrefix:n,arrowStyle:this.arrowStyle,arrowClass:void 0,arrowWrapperClass:void 0,arrowWrapperStyle:void 0}):null)}}),yr=K("dropdown-menu",`
 transform-origin: var(--v-transform-origin);
 background-color: var(--n-color);
 border-radius: var(--n-border-radius);
 box-shadow: var(--n-box-shadow);
 position: relative;
 transition:
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
`,[Pn(),K("dropdown-option",`
 position: relative;
 `,[se("a",`
 text-decoration: none;
 color: inherit;
 outline: none;
 `,[se("&::before",`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),K("dropdown-option-body",`
 display: flex;
 cursor: pointer;
 position: relative;
 height: var(--n-option-height);
 line-height: var(--n-option-height);
 font-size: var(--n-font-size);
 color: var(--n-option-text-color);
 transition: color .3s var(--n-bezier);
 `,[se("&::before",`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 left: 4px;
 right: 4px;
 transition: background-color .3s var(--n-bezier);
 border-radius: var(--n-border-radius);
 `),Le("disabled",[ne("pending",`
 color: var(--n-option-text-color-hover);
 `,[G("prefix, suffix",`
 color: var(--n-option-text-color-hover);
 `),se("&::before","background-color: var(--n-option-color-hover);")]),ne("active",`
 color: var(--n-option-text-color-active);
 `,[G("prefix, suffix",`
 color: var(--n-option-text-color-active);
 `),se("&::before","background-color: var(--n-option-color-active);")]),ne("child-active",`
 color: var(--n-option-text-color-child-active);
 `,[G("prefix, suffix",`
 color: var(--n-option-text-color-child-active);
 `)])]),ne("disabled",`
 cursor: not-allowed;
 opacity: var(--n-option-opacity-disabled);
 `),ne("group",`
 font-size: calc(var(--n-font-size) - 1px);
 color: var(--n-group-header-text-color);
 `,[G("prefix",`
 width: calc(var(--n-option-prefix-width) / 2);
 `,[ne("show-icon",`
 width: calc(var(--n-option-icon-prefix-width) / 2);
 `)])]),G("prefix",`
 width: var(--n-option-prefix-width);
 display: flex;
 justify-content: center;
 align-items: center;
 color: var(--n-prefix-color);
 transition: color .3s var(--n-bezier);
 z-index: 1;
 `,[ne("show-icon",`
 width: var(--n-option-icon-prefix-width);
 `),K("icon",`
 font-size: var(--n-option-icon-size);
 `)]),G("label",`
 white-space: nowrap;
 flex: 1;
 z-index: 1;
 `),G("suffix",`
 box-sizing: border-box;
 flex-grow: 0;
 flex-shrink: 0;
 display: flex;
 justify-content: flex-end;
 align-items: center;
 min-width: var(--n-option-suffix-width);
 padding: 0 8px;
 transition: color .3s var(--n-bezier);
 color: var(--n-suffix-color);
 z-index: 1;
 `,[ne("has-submenu",`
 width: var(--n-option-icon-suffix-width);
 `),K("icon",`
 font-size: var(--n-option-icon-size);
 `)]),K("dropdown-menu","pointer-events: all;")]),K("dropdown-offset-container",`
 pointer-events: none;
 position: absolute;
 left: 0;
 right: 0;
 top: -4px;
 bottom: -4px;
 `)]),K("dropdown-divider",`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 4px 0;
 `),K("dropdown-menu-wrapper",`
 transform-origin: var(--v-transform-origin);
 width: fit-content;
 `),se(">",[K("scrollbar",`
 height: inherit;
 max-height: inherit;
 `)]),Le("scrollable",`
 padding: var(--n-padding);
 `),ne("scrollable",[G("content",`
 padding: var(--n-padding);
 `)])]),wr={animated:{type:Boolean,default:!0},keyboard:{type:Boolean,default:!0},size:String,inverted:Boolean,placement:{type:String,default:"bottom"},onSelect:[Function,Array],options:{type:Array,default:()=>[]},menuProps:Function,showArrow:Boolean,renderLabel:Function,renderIcon:Function,renderOption:Function,nodeProps:Function,labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},value:[String,Number]},xr=Object.keys(et),Sr=Object.assign(Object.assign(Object.assign({},et),wr),we.props),Or=ae({name:"Dropdown",inheritAttrs:!1,props:Sr,setup(e){const n=B(!1),t=wn(Z(e,"show"),n),o=N(()=>{const{keyField:g,childrenField:k}=e;return lt(e.options,{getKey(P){return P[g]},getDisabled(P){return P.disabled===!0},getIgnored(P){return P.type==="divider"||P.type==="render"},getChildren(P){return P[k]}})}),r=N(()=>o.value.treeNodes),i=B(null),l=B(null),a=B(null),d=N(()=>{var g,k,P;return(P=(k=(g=i.value)!==null&&g!==void 0?g:l.value)!==null&&k!==void 0?k:a.value)!==null&&P!==void 0?P:null}),c=N(()=>o.value.getPath(d.value).keyPath),f=N(()=>o.value.getPath(e.value).keyPath),h=pe(()=>e.keyboard&&t.value);wo({keydown:{ArrowUp:{prevent:!0,handler:H},ArrowRight:{prevent:!0,handler:A},ArrowDown:{prevent:!0,handler:q},ArrowLeft:{prevent:!0,handler:S},Enter:{prevent:!0,handler:J},Escape:w}},h);const{mergedClsPrefixRef:O,inlineThemeDisabled:T,mergedComponentPropsRef:m}=je(e),$=N(()=>{var g,k;return e.size||((k=(g=m==null?void 0:m.value)===null||g===void 0?void 0:g.Dropdown)===null||k===void 0?void 0:k.size)||"medium"}),D=we("Dropdown","-dropdown",yr,co,e,O);Se(on,{labelFieldRef:Z(e,"labelField"),childrenFieldRef:Z(e,"childrenField"),renderLabelRef:Z(e,"renderLabel"),renderIconRef:Z(e,"renderIcon"),hoverKeyRef:i,keyboardKeyRef:l,lastToggledSubmenuKeyRef:a,pendingKeyPathRef:c,activeKeyPathRef:f,animatedRef:Z(e,"animated"),mergedShowRef:t,nodePropsRef:Z(e,"nodeProps"),renderOptionRef:Z(e,"renderOption"),menuPropsRef:Z(e,"menuProps"),doSelect:F,doUpdateShow:I}),ye(t,g=>{!e.animated&&!g&&_()});function F(g,k){const{onSelect:P}=e;P&&fe(P,g,k)}function I(g){const{"onUpdate:show":k,onUpdateShow:P}=e;k&&fe(k,g),P&&fe(P,g),n.value=g}function _(){i.value=null,l.value=null,a.value=null}function w(){I(!1)}function S(){Q("left")}function A(){Q("right")}function H(){Q("up")}function q(){Q("down")}function J(){const g=V();g!=null&&g.isLeaf&&t.value&&(F(g.key,g.rawNode),I(!1))}function V(){var g;const{value:k}=o,{value:P}=d;return!k||P===null?null:(g=k.getNode(P))!==null&&g!==void 0?g:null}function Q(g){const{value:k}=d,{value:{getFirstAvailableNode:P}}=o;let y=null;if(k===null){const z=P();z!==null&&(y=z.key)}else{const z=V();if(z){let E;switch(g){case"down":E=z.getNext();break;case"up":E=z.getPrev();break;case"right":E=z.getChild();break;case"left":E=z.getParent();break}E&&(y=E.key)}}y!==null&&(i.value=null,l.value=y)}const ie=N(()=>{const{inverted:g}=e,k=$.value,{common:{cubicBezierEaseInOut:P},self:y}=D.value,{padding:z,dividerColor:E,borderRadius:Y,optionOpacityDisabled:ee,[ue("optionIconSuffixWidth",k)]:te,[ue("optionSuffixWidth",k)]:de,[ue("optionIconPrefixWidth",k)]:p,[ue("optionPrefixWidth",k)]:C,[ue("fontSize",k)]:oe,[ue("optionHeight",k)]:he,[ue("optionIconSize",k)]:Ce}=y,X={"--n-bezier":P,"--n-font-size":oe,"--n-padding":z,"--n-border-radius":Y,"--n-option-height":he,"--n-option-prefix-width":C,"--n-option-icon-prefix-width":p,"--n-option-suffix-width":de,"--n-option-icon-suffix-width":te,"--n-option-icon-size":Ce,"--n-divider-color":E,"--n-option-opacity-disabled":ee};return g?(X["--n-color"]=y.colorInverted,X["--n-option-color-hover"]=y.optionColorHoverInverted,X["--n-option-color-active"]=y.optionColorActiveInverted,X["--n-option-text-color"]=y.optionTextColorInverted,X["--n-option-text-color-hover"]=y.optionTextColorHoverInverted,X["--n-option-text-color-active"]=y.optionTextColorActiveInverted,X["--n-option-text-color-child-active"]=y.optionTextColorChildActiveInverted,X["--n-prefix-color"]=y.prefixColorInverted,X["--n-suffix-color"]=y.suffixColorInverted,X["--n-group-header-text-color"]=y.groupHeaderTextColorInverted):(X["--n-color"]=y.color,X["--n-option-color-hover"]=y.optionColorHover,X["--n-option-color-active"]=y.optionColorActive,X["--n-option-text-color"]=y.optionTextColor,X["--n-option-text-color-hover"]=y.optionTextColorHover,X["--n-option-text-color-active"]=y.optionTextColorActive,X["--n-option-text-color-child-active"]=y.optionTextColorChildActive,X["--n-prefix-color"]=y.prefixColor,X["--n-suffix-color"]=y.suffixColor,X["--n-group-header-text-color"]=y.groupHeaderTextColor),X}),v=T?We("dropdown",N(()=>`${$.value[0]}${e.inverted?"i":""}`),ie,e):void 0;return{mergedClsPrefix:O,mergedTheme:D,mergedSize:$,tmNodes:r,mergedShow:t,handleAfterLeave:()=>{e.animated&&_()},doUpdateShow:I,cssVars:T?void 0:ie,themeClass:v==null?void 0:v.themeClass,onRender:v==null?void 0:v.onRender}},render(){const e=(o,r,i,l,a)=>{var d;const{mergedClsPrefix:c,menuProps:f}=this;(d=this.onRender)===null||d===void 0||d.call(this);const h=(f==null?void 0:f(void 0,this.tmNodes.map(T=>T.rawNode)))||{},O={ref:Oo(r),class:[o,`${c}-dropdown`,`${c}-dropdown--${this.mergedSize}-size`,this.themeClass],clsPrefix:c,tmNodes:this.tmNodes,style:[...i,this.cssVars],showArrow:this.showArrow,arrowStyle:this.arrowStyle,scrollable:this.scrollable,onMouseenter:l,onMouseleave:a};return u(ct,tn(this.$attrs,O,h))},{mergedTheme:n}=this,t={show:this.mergedShow,theme:n.peers.Popover,themeOverrides:n.peerOverrides.Popover,internalOnAfterLeave:this.handleAfterLeave,internalRenderBody:e,onUpdateShow:this.doUpdateShow,"onUpdate:show":void 0};return u(Yn,Object.assign({},uo(this.$props,xr),t),{trigger:()=>{var o,r;return(r=(o=this.$slots).default)===null||r===void 0?void 0:r.call(o)}})}});export{Fo as C,Io as F,Or as N,$n as V,Pr as a,ko as b,lt as c,or as d,lr as e,Oo as f,Te as h,hn as m,wo as u};
