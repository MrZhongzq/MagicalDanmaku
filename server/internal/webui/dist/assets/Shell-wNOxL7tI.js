import{P as wn,Q as Cn,o as Ht,R as it,S as Sn,U as Rn,V as lt,w as Re,r as j,W as be,n as I,p as pe,e as re,i as ue,h as d,X as Ft,Y as Ze,v as He,Z as zn,_ as In,$ as _t,a0 as yo,t as ie,a1 as qe,a2 as Dt,a3 as Pn,a4 as xt,a5 as he,a6 as jt,T as Vt,a as S,m as k,b as W,d as G,a7 as Se,a8 as Wt,q as ro,a9 as kn,aa as Ut,ab as Tn,u as Ie,ac as xo,f as fe,x as Pe,ad as On,A as ge,ae as Ye,af as Nn,ag as Gt,ah as At,ai as Fn,aj as _n,ak as An,al as io,am as Mn,an as Bn,ao as $n,ap as En,aq as Ln,ar as ae,c as Oe,z as wo,as as Kn,at as Co,au as Hn,av as Dn,aw as jn,ax as dt,ay as Vn,az as Wn,aA as Un,aB as Gn,aC as lo,aD as So,aE as qn,k as qt,aF as Yn,s as Xn,aG as Zn,B as Jn,aH as wt,E as Le,F as ve,H as Ct,D as _e,M as St,aI as Qn,K as er,L as tr,aJ as or,aK as nr,G as rr}from"./index-DLF9QZAV.js";import{c as Ro,b as ir,a as ct,i as Yt,N as lr,d as ar,e as Rt,f as zo,B as Io,V as Po,g as ko,u as Mt,h as To,r as sr,p as Oo,j as dr,k as cr,l as ur}from"./bindings-3FPvhT6-.js";import{N as fr,a as Ke,b as hr,f as ut,u as vr,_ as pr}from"./_plugin-vue_export-helper-BrAt0rlp.js";function Me(e,t){let{target:o}=e;for(;o;){if(o.dataset&&o.dataset[t]!==void 0)return!0;o=o.parentElement}return!1}function mr(e={},t){const o=Rn({ctrl:!1,command:!1,win:!1,shift:!1,tab:!1}),{keydown:n,keyup:r}=e,i=s=>{switch(s.key){case"Control":o.ctrl=!0;break;case"Meta":o.command=!0,o.win=!0;break;case"Shift":o.shift=!0;break;case"Tab":o.tab=!0;break}n!==void 0&&Object.keys(n).forEach(c=>{if(c!==s.key)return;const h=n[c];if(typeof h=="function")h(s);else{const{stop:p=!1,prevent:b=!1}=h;p&&s.stopPropagation(),b&&s.preventDefault(),h.handler(s)}})},l=s=>{switch(s.key){case"Control":o.ctrl=!1;break;case"Meta":o.command=!1,o.win=!1;break;case"Shift":o.shift=!1;break;case"Tab":o.tab=!1;break}r!==void 0&&Object.keys(r).forEach(c=>{if(c!==s.key)return;const h=r[c];if(typeof h=="function")h(s);else{const{stop:p=!1,prevent:b=!1}=h;p&&s.stopPropagation(),b&&s.preventDefault(),h.handler(s)}})},a=()=>{(t===void 0||t.value)&&(lt("keydown",document,i),lt("keyup",document,l)),t!==void 0&&Re(t,s=>{s?(lt("keydown",document,i),lt("keyup",document,l)):(it("keydown",document,i),it("keyup",document,l))})};return wn()?(Cn(a),Ht(()=>{(t===void 0||t.value)&&(it("keydown",document,i),it("keyup",document,l))})):a(),Sn(o)}function gr(e,t,o){const n=j(e.value);let r=null;return Re(e,i=>{r!==null&&window.clearTimeout(r),i===!0?o&&!o.value?n.value=!0:r=window.setTimeout(()=>{n.value=!0},t):n.value=!1}),n}function ao(e){return e&-e}class No{constructor(t,o){this.l=t,this.min=o;const n=new Array(t+1);for(let r=0;r<t+1;++r)n[r]=0;this.ft=n}add(t,o){if(o===0)return;const{l:n,ft:r}=this;for(t+=1;t<=n;)r[t]+=o,t+=ao(t)}get(t){return this.sum(t+1)-this.sum(t)}sum(t){if(t===void 0&&(t=this.l),t<=0)return 0;const{ft:o,min:n,l:r}=this;if(t>r)throw new Error("[FinweckTree.sum]: `i` is larger than length.");let i=t*n;for(;t>0;)i+=o[t],t-=ao(t);return i}getBound(t){let o=0,n=this.l;for(;n>o;){const r=Math.floor((o+n)/2),i=this.sum(r);if(i>t){n=r;continue}else if(i<t){if(o===r)return this.sum(o+1)<=t?o+1:r;o=r}else return r}return o}}let at;function br(){return typeof document>"u"?!1:(at===void 0&&("matchMedia"in window?at=window.matchMedia("(pointer:coarse)").matches:at=!1),at)}let zt;function so(){return typeof document>"u"?1:(zt===void 0&&(zt="chrome"in window?window.devicePixelRatio:1),zt)}const Fo="VVirtualListXScroll";function yr({columnsRef:e,renderColRef:t,renderItemWithColsRef:o}){const n=j(0),r=j(0),i=I(()=>{const c=e.value;if(c.length===0)return null;const h=new No(c.length,0);return c.forEach((p,b)=>{h.add(b,p.width)}),h}),l=be(()=>{const c=i.value;return c!==null?Math.max(c.getBound(r.value)-1,0):0}),a=c=>{const h=i.value;return h!==null?h.sum(c):0},s=be(()=>{const c=i.value;return c!==null?Math.min(c.getBound(r.value+n.value)+1,e.value.length-1):0});return pe(Fo,{startIndexRef:l,endIndexRef:s,columnsRef:e,renderColRef:t,renderItemWithColsRef:o,getLeft:a}),{listWidthRef:n,scrollLeftRef:r}}const co=re({name:"VirtualListRow",props:{index:{type:Number,required:!0},item:{type:Object,required:!0}},setup(){const{startIndexRef:e,endIndexRef:t,columnsRef:o,getLeft:n,renderColRef:r,renderItemWithColsRef:i}=ue(Fo);return{startIndex:e,endIndex:t,columns:o,renderCol:r,renderItemWithCols:i,getLeft:n}},render(){const{startIndex:e,endIndex:t,columns:o,renderCol:n,renderItemWithCols:r,getLeft:i,item:l}=this;if(r!=null)return r({itemIndex:this.index,startColIndex:e,endColIndex:t,allColumns:o,item:l,getLeft:i});if(n!=null){const a=[];for(let s=e;s<=t;++s){const c=o[s];a.push(n({column:c,left:i(s),item:l}))}return a}return null}}),xr=ct(".v-vl",{maxHeight:"inherit",height:"100%",overflow:"auto",minWidth:"1px"},[ct("&:not(.v-vl--show-scrollbar)",{scrollbarWidth:"none"},[ct("&::-webkit-scrollbar, &::-webkit-scrollbar-track-piece, &::-webkit-scrollbar-thumb",{width:0,height:0,display:"none"})])]),wr=re({name:"VirtualList",inheritAttrs:!1,props:{showScrollbar:{type:Boolean,default:!0},columns:{type:Array,default:()=>[]},renderCol:Function,renderItemWithCols:Function,items:{type:Array,default:()=>[]},itemSize:{type:Number,required:!0},itemResizable:Boolean,itemsStyle:[String,Object],visibleItemsTag:{type:[String,Object],default:"div"},visibleItemsProps:Object,ignoreItemResize:Boolean,onScroll:Function,onWheel:Function,onResize:Function,defaultScrollKey:[Number,String],defaultScrollIndex:Number,keyField:{type:String,default:"key"},paddingTop:{type:[Number,String],default:0},paddingBottom:{type:[Number,String],default:0}},setup(e){const t=yo();xr.mount({id:"vueuc/virtual-list",head:!0,anchorMetaName:Ro,ssr:t}),He(()=>{const{defaultScrollIndex:C,defaultScrollKey:x}=e;C!=null?B({index:C}):x!=null&&B({key:x})});let o=!1,n=!1;zn(()=>{if(o=!1,!n){n=!0;return}B({top:O.value,left:l.value})}),In(()=>{o=!0,n||(n=!0)});const r=be(()=>{if(e.renderCol==null&&e.renderItemWithCols==null||e.columns.length===0)return;let C=0;return e.columns.forEach(x=>{C+=x.width}),C}),i=I(()=>{const C=new Map,{keyField:x}=e;return e.items.forEach(($,L)=>{C.set($[x],L)}),C}),{scrollLeftRef:l,listWidthRef:a}=yr({columnsRef:ie(e,"columns"),renderColRef:ie(e,"renderCol"),renderItemWithColsRef:ie(e,"renderItemWithCols")}),s=j(null),c=j(void 0),h=new Map,p=I(()=>{const{items:C,itemSize:x,keyField:$}=e,L=new No(C.length,x);return C.forEach((P,V)=>{const q=P[$],ne=h.get(q);ne!==void 0&&L.add(V,ne)}),L}),b=j(0),O=j(0),g=be(()=>Math.max(p.value.getBound(O.value-_t(e.paddingTop))-1,0)),_=I(()=>{const{value:C}=c;if(C===void 0)return[];const{items:x,itemSize:$}=e,L=g.value,P=Math.min(L+Math.ceil(C/$+1),x.length-1),V=[];for(let q=L;q<=P;++q)V.push(x[q]);return V}),B=(C,x)=>{if(typeof C=="number"){R(C,x,"auto");return}const{left:$,top:L,index:P,key:V,position:q,behavior:ne,debounce:A=!0}=C;if($!==void 0||L!==void 0)R($,L,ne);else if(P!==void 0)H(P,ne,A);else if(V!==void 0){const D=i.value.get(V);D!==void 0&&H(D,ne,A)}else q==="bottom"?R(0,Number.MAX_SAFE_INTEGER,ne):q==="top"&&R(0,0,ne)};let N,K=null;function H(C,x,$){const{value:L}=p,P=L.sum(C)+_t(e.paddingTop);if(!$)s.value.scrollTo({left:0,top:P,behavior:x});else{N=C,K!==null&&window.clearTimeout(K),K=window.setTimeout(()=>{N=void 0,K=null},16);const{scrollTop:V,offsetHeight:q}=s.value;if(P>V){const ne=L.get(C);P+ne<=V+q||s.value.scrollTo({left:0,top:P+ne-q,behavior:x})}else s.value.scrollTo({left:0,top:P,behavior:x})}}function R(C,x,$){s.value.scrollTo({left:C,top:x,behavior:$})}function y(C,x){var $,L,P;if(o||e.ignoreItemResize||se(x.target))return;const{value:V}=p,q=i.value.get(C),ne=V.get(q),A=(P=(L=($=x.borderBoxSize)===null||$===void 0?void 0:$[0])===null||L===void 0?void 0:L.blockSize)!==null&&P!==void 0?P:x.contentRect.height;if(A===ne)return;A-e.itemSize===0?h.delete(C):h.set(C,A-e.itemSize);const v=A-ne;if(v===0)return;V.add(q,v);const f=s.value;if(f!=null){if(N===void 0){const w=V.sum(q);f.scrollTop>w&&f.scrollBy(0,v)}else if(q<N)f.scrollBy(0,v);else if(q===N){const w=V.sum(q);A+w>f.scrollTop+f.offsetHeight&&f.scrollBy(0,v)}Z()}b.value++}const T=!br();let M=!1;function Y(C){var x;(x=e.onScroll)===null||x===void 0||x.call(e,C),(!T||!M)&&Z()}function te(C){var x;if((x=e.onWheel)===null||x===void 0||x.call(e,C),T){const $=s.value;if($!=null){if(C.deltaX===0&&($.scrollTop===0&&C.deltaY<=0||$.scrollTop+$.offsetHeight>=$.scrollHeight&&C.deltaY>=0))return;C.preventDefault(),$.scrollTop+=C.deltaY/so(),$.scrollLeft+=C.deltaX/so(),Z(),M=!0,ir(()=>{M=!1})}}}function J(C){if(o||se(C.target))return;if(e.renderCol==null&&e.renderItemWithCols==null){if(C.contentRect.height===c.value)return}else if(C.contentRect.height===c.value&&C.contentRect.width===a.value)return;c.value=C.contentRect.height,a.value=C.contentRect.width;const{onResize:x}=e;x!==void 0&&x(C)}function Z(){const{value:C}=s;C!=null&&(O.value=C.scrollTop,l.value=C.scrollLeft)}function se(C){let x=C;for(;x!==null;){if(x.style.display==="none")return!0;x=x.parentElement}return!1}return{listHeight:c,listStyle:{overflow:"auto"},keyToIndex:i,itemsStyle:I(()=>{const{itemResizable:C}=e,x=qe(p.value.sum());return b.value,[e.itemsStyle,{boxSizing:"content-box",width:qe(r.value),height:C?"":x,minHeight:C?x:"",paddingTop:qe(e.paddingTop),paddingBottom:qe(e.paddingBottom)}]}),visibleItemsStyle:I(()=>(b.value,{transform:`translateY(${qe(p.value.sum(g.value))})`})),viewportItems:_,listElRef:s,itemsElRef:j(null),scrollTo:B,handleListResize:J,handleListScroll:Y,handleListWheel:te,handleItemResize:y}},render(){const{itemResizable:e,keyField:t,keyToIndex:o,visibleItemsTag:n}=this;return d(Ft,{onResize:this.handleListResize},{default:()=>{var r,i;return d("div",Ze(this.$attrs,{class:["v-vl",this.showScrollbar&&"v-vl--show-scrollbar"],onScroll:this.handleListScroll,onWheel:this.handleListWheel,ref:"listElRef"}),[this.items.length!==0?d("div",{ref:"itemsElRef",class:"v-vl-items",style:this.itemsStyle},[d(n,Object.assign({class:"v-vl-visible-items",style:this.visibleItemsStyle},this.visibleItemsProps),{default:()=>{const{renderCol:l,renderItemWithCols:a}=this;return this.viewportItems.map(s=>{const c=s[t],h=o.get(c),p=l!=null?d(co,{index:h,item:s}):void 0,b=a!=null?d(co,{index:h,item:s}):void 0,O=this.$slots.default({item:s,renderedCols:p,renderedItemWithCols:b,index:h})[0];return e?d(Ft,{key:c,onResize:g=>this.handleItemResize(c,g)},{default:()=>O}):(O.key=c,O)})}})]):(i=(r=this.$slots).empty)===null||i===void 0?void 0:i.call(r)])}})}}),ze="v-hidden",Cr=ct("[v-hidden]",{display:"none!important"}),Bt=re({name:"Overflow",props:{getCounter:Function,getTail:Function,updateCounter:Function,onUpdateCount:Function,onUpdateOverflow:Function},setup(e,{slots:t}){const o=j(null),n=j(null);function r(l){const{value:a}=o,{getCounter:s,getTail:c}=e;let h;if(s!==void 0?h=s():h=n.value,!a||!h)return;h.hasAttribute(ze)&&h.removeAttribute(ze);const{children:p}=a;if(l.showAllItemsBeforeCalculate)for(const H of p)H.hasAttribute(ze)&&H.removeAttribute(ze);const b=a.offsetWidth,O=[],g=t.tail?c==null?void 0:c():null;let _=g?g.offsetWidth:0,B=!1;const N=a.children.length-(t.tail?1:0);for(let H=0;H<N-1;++H){if(H<0)continue;const R=p[H];if(B){R.hasAttribute(ze)||R.setAttribute(ze,"");continue}else R.hasAttribute(ze)&&R.removeAttribute(ze);const y=R.offsetWidth;if(_+=y,O[H]=y,_>b){const{updateCounter:T}=e;for(let M=H;M>=0;--M){const Y=N-1-M;T!==void 0?T(Y):h.textContent=`${Y}`;const te=h.offsetWidth;if(_-=O[M],_+te<=b||M===0){B=!0,H=M-1,g&&(H===-1?(g.style.maxWidth=`${b-te}px`,g.style.boxSizing="border-box"):g.style.maxWidth="");const{onUpdateCount:J}=e;J&&J(Y);break}}}}const{onUpdateOverflow:K}=e;B?K!==void 0&&K(!0):(K!==void 0&&K(!1),h.setAttribute(ze,""))}const i=yo();return Cr.mount({id:"vueuc/overflow",head:!0,anchorMetaName:Ro,ssr:i}),He(()=>r({showAllItemsBeforeCalculate:!1})),{selfRef:o,counterRef:n,sync:r}},render(){const{$slots:e}=this;return Dt(()=>this.sync({showAllItemsBeforeCalculate:!1})),d("div",{class:"v-overflow",ref:"selfRef"},[Pn(e,"default"),e.counter?e.counter():d("span",{style:{display:"inline-block"},ref:"counterRef"}),e.tail?e.tail():null])}});function _o(e,t){t&&(He(()=>{const{value:o}=e;o&&xt.registerHandler(o,t)}),Re(e,(o,n)=>{n&&xt.unregisterHandler(n)},{deep:!1}),Ht(()=>{const{value:o}=e;o&&xt.unregisterHandler(o)}))}function uo(e){switch(typeof e){case"string":return e||void 0;case"number":return String(e);default:return}}function Sr(e){return t=>{t?e.value=t.$el:e.value=null}}function It(e){const t=e.filter(o=>o!==void 0);if(t.length!==0)return t.length===1?t[0]:o=>{e.forEach(n=>{n&&n(o)})}}const Rr=re({name:"Checkmark",render(){return d("svg",{xmlns:"http://www.w3.org/2000/svg",viewBox:"0 0 16 16"},d("g",{fill:"none"},d("path",{d:"M14.046 3.486a.75.75 0 0 1-.032 1.06l-7.93 7.474a.85.85 0 0 1-1.188-.022l-2.68-2.72a.75.75 0 1 1 1.068-1.053l2.234 2.267l7.468-7.038a.75.75 0 0 1 1.06.032z",fill:"currentColor"})))}}),zr=re({name:"ChevronDownFilled",render(){return d("svg",{viewBox:"0 0 16 16",fill:"none",xmlns:"http://www.w3.org/2000/svg"},d("path",{d:"M3.20041 5.73966C3.48226 5.43613 3.95681 5.41856 4.26034 5.70041L8 9.22652L11.7397 5.70041C12.0432 5.41856 12.5177 5.43613 12.7996 5.73966C13.0815 6.0432 13.0639 6.51775 12.7603 6.7996L8.51034 10.7996C8.22258 11.0668 7.77743 11.0668 7.48967 10.7996L3.23966 6.7996C2.93613 6.51775 2.91856 6.0432 3.20041 5.73966Z",fill:"currentColor"}))}}),Ao=re({name:"ChevronRight",render(){return d("svg",{viewBox:"0 0 16 16",fill:"none",xmlns:"http://www.w3.org/2000/svg"},d("path",{d:"M5.64645 3.14645C5.45118 3.34171 5.45118 3.65829 5.64645 3.85355L9.79289 8L5.64645 12.1464C5.45118 12.3417 5.45118 12.6583 5.64645 12.8536C5.84171 13.0488 6.15829 13.0488 6.35355 12.8536L10.8536 8.35355C11.0488 8.15829 11.0488 7.84171 10.8536 7.64645L6.35355 3.14645C6.15829 2.95118 5.84171 2.95118 5.64645 3.14645Z",fill:"currentColor"}))}}),Ir=re({props:{onFocus:Function,onBlur:Function},setup(e){return()=>d("div",{style:"width: 0; height: 0",tabindex:0,onFocus:e.onFocus,onBlur:e.onBlur})}});function fo(e){return Array.isArray(e)?e:[e]}const $t={STOP:"STOP"};function Mo(e,t){const o=t(e);e.children!==void 0&&o!==$t.STOP&&e.children.forEach(n=>Mo(n,t))}function Pr(e,t={}){const{preserveGroup:o=!1}=t,n=[],r=o?l=>{l.isLeaf||(n.push(l.key),i(l.children))}:l=>{l.isLeaf||(l.isGroup||n.push(l.key),i(l.children))};function i(l){l.forEach(r)}return i(e),n}function kr(e,t){const{isLeaf:o}=e;return o!==void 0?o:!t(e)}function Tr(e){return e.children}function Or(e){return e.key}function Nr(){return!1}function Fr(e,t){const{isLeaf:o}=e;return!(o===!1&&!Array.isArray(t(e)))}function _r(e){return e.disabled===!0}function Ar(e,t){return e.isLeaf===!1&&!Array.isArray(t(e))}function Pt(e){var t;return e==null?[]:Array.isArray(e)?e:(t=e.checkedKeys)!==null&&t!==void 0?t:[]}function kt(e){var t;return e==null||Array.isArray(e)?[]:(t=e.indeterminateKeys)!==null&&t!==void 0?t:[]}function Mr(e,t){const o=new Set(e);return t.forEach(n=>{o.has(n)||o.add(n)}),Array.from(o)}function Br(e,t){const o=new Set(e);return t.forEach(n=>{o.has(n)&&o.delete(n)}),Array.from(o)}function $r(e){return(e==null?void 0:e.type)==="group"}function Er(e){const t=new Map;return e.forEach((o,n)=>{t.set(o.key,n)}),o=>{var n;return(n=t.get(o))!==null&&n!==void 0?n:null}}class Lr extends Error{constructor(){super(),this.message="SubtreeNotLoadedError: checking a subtree whose required nodes are not fully loaded."}}function Kr(e,t,o,n){return ft(t.concat(e),o,n,!1)}function Hr(e,t){const o=new Set;return e.forEach(n=>{const r=t.treeNodeMap.get(n);if(r!==void 0){let i=r.parent;for(;i!==null&&!(i.disabled||o.has(i.key));)o.add(i.key),i=i.parent}}),o}function Dr(e,t,o,n){const r=ft(t,o,n,!1),i=ft(e,o,n,!0),l=Hr(e,o),a=[];return r.forEach(s=>{(i.has(s)||l.has(s))&&a.push(s)}),a.forEach(s=>r.delete(s)),r}function Tt(e,t){const{checkedKeys:o,keysToCheck:n,keysToUncheck:r,indeterminateKeys:i,cascade:l,leafOnly:a,checkStrategy:s,allowNotLoaded:c}=e;if(!l)return n!==void 0?{checkedKeys:Mr(o,n),indeterminateKeys:Array.from(i)}:r!==void 0?{checkedKeys:Br(o,r),indeterminateKeys:Array.from(i)}:{checkedKeys:Array.from(o),indeterminateKeys:Array.from(i)};const{levelTreeNodeMap:h}=t;let p;r!==void 0?p=Dr(r,o,t,c):n!==void 0?p=Kr(n,o,t,c):p=ft(o,t,c,!1);const b=s==="parent",O=s==="child"||a,g=p,_=new Set,B=Math.max.apply(null,Array.from(h.keys()));for(let N=B;N>=0;N-=1){const K=N===0,H=h.get(N);for(const R of H){if(R.isLeaf)continue;const{key:y,shallowLoaded:T}=R;if(O&&T&&R.children.forEach(J=>{!J.disabled&&!J.isLeaf&&J.shallowLoaded&&g.has(J.key)&&g.delete(J.key)}),R.disabled||!T)continue;let M=!0,Y=!1,te=!0;for(const J of R.children){const Z=J.key;if(!J.disabled){if(te&&(te=!1),g.has(Z))Y=!0;else if(_.has(Z)){Y=!0,M=!1;break}else if(M=!1,Y)break}}M&&!te?(b&&R.children.forEach(J=>{!J.disabled&&g.has(J.key)&&g.delete(J.key)}),g.add(y)):Y&&_.add(y),K&&O&&g.has(y)&&g.delete(y)}}return{checkedKeys:Array.from(g),indeterminateKeys:Array.from(_)}}function ft(e,t,o,n){const{treeNodeMap:r,getChildren:i}=t,l=new Set,a=new Set(e);return e.forEach(s=>{const c=r.get(s);c!==void 0&&Mo(c,h=>{if(h.disabled)return $t.STOP;const{key:p}=h;if(!l.has(p)&&(l.add(p),a.add(p),Ar(h.rawNode,i))){if(n)return $t.STOP;if(!o)throw new Lr}})}),a}function jr(e,{includeGroup:t=!1,includeSelf:o=!0},n){var r;const i=n.treeNodeMap;let l=e==null?null:(r=i.get(e))!==null&&r!==void 0?r:null;const a={keyPath:[],treeNodePath:[],treeNode:l};if(l!=null&&l.ignored)return a.treeNode=null,a;for(;l;)!l.ignored&&(t||!l.isGroup)&&a.treeNodePath.push(l),l=l.parent;return a.treeNodePath.reverse(),o||a.treeNodePath.pop(),a.keyPath=a.treeNodePath.map(s=>s.key),a}function Vr(e){if(e.length===0)return null;const t=e[0];return t.isGroup||t.ignored||t.disabled?t.getNext():t}function Wr(e,t){const o=e.siblings,n=o.length,{index:r}=e;return t?o[(r+1)%n]:r===o.length-1?null:o[r+1]}function ho(e,t,{loop:o=!1,includeDisabled:n=!1}={}){const r=t==="prev"?Ur:Wr,i={reverse:t==="prev"};let l=!1,a=null;function s(c){if(c!==null){if(c===e){if(!l)l=!0;else if(!e.disabled&&!e.isGroup){a=e;return}}else if((!c.disabled||n)&&!c.ignored&&!c.isGroup){a=c;return}if(c.isGroup){const h=Xt(c,i);h!==null?a=h:s(r(c,o))}else{const h=r(c,!1);if(h!==null)s(h);else{const p=Gr(c);p!=null&&p.isGroup?s(r(p,o)):o&&s(r(c,!0))}}}}return s(e),a}function Ur(e,t){const o=e.siblings,n=o.length,{index:r}=e;return t?o[(r-1+n)%n]:r===0?null:o[r-1]}function Gr(e){return e.parent}function Xt(e,t={}){const{reverse:o=!1}=t,{children:n}=e;if(n){const{length:r}=n,i=o?r-1:0,l=o?-1:r,a=o?-1:1;for(let s=i;s!==l;s+=a){const c=n[s];if(!c.disabled&&!c.ignored)if(c.isGroup){const h=Xt(c,t);if(h!==null)return h}else return c}}return null}const qr={getChild(){return this.ignored?null:Xt(this)},getParent(){const{parent:e}=this;return e!=null&&e.isGroup?e.getParent():e},getNext(e={}){return ho(this,"next",e)},getPrev(e={}){return ho(this,"prev",e)}};function Yr(e,t){const o=t?new Set(t):void 0,n=[];function r(i){i.forEach(l=>{n.push(l),!(l.isLeaf||!l.children||l.ignored)&&(l.isGroup||o===void 0||o.has(l.key))&&r(l.children)})}return r(e),n}function Xr(e,t){const o=e.key;for(;t;){if(t.key===o)return!0;t=t.parent}return!1}function Bo(e,t,o,n,r,i=null,l=0){const a=[];return e.forEach((s,c)=>{var h;const p=Object.create(n);if(p.rawNode=s,p.siblings=a,p.level=l,p.index=c,p.isFirstChild=c===0,p.isLastChild=c+1===e.length,p.parent=i,!p.ignored){const b=r(s);Array.isArray(b)&&(p.children=Bo(b,t,o,n,r,p,l+1))}a.push(p),t.set(p.key,p),o.has(l)||o.set(l,[]),(h=o.get(l))===null||h===void 0||h.push(p)}),a}function Xe(e,t={}){var o;const n=new Map,r=new Map,{getDisabled:i=_r,getIgnored:l=Nr,getIsGroup:a=$r,getKey:s=Or}=t,c=(o=t.getChildren)!==null&&o!==void 0?o:Tr,h=t.ignoreEmptyChildren?R=>{const y=c(R);return Array.isArray(y)?y.length?y:null:y}:c,p=Object.assign({get key(){return s(this.rawNode)},get disabled(){return i(this.rawNode)},get isGroup(){return a(this.rawNode)},get isLeaf(){return kr(this.rawNode,h)},get shallowLoaded(){return Fr(this.rawNode,h)},get ignored(){return l(this.rawNode)},contains(R){return Xr(this,R)}},qr),b=Bo(e,n,r,p,h);function O(R){if(R==null)return null;const y=n.get(R);return y&&!y.isGroup&&!y.ignored?y:null}function g(R){if(R==null)return null;const y=n.get(R);return y&&!y.ignored?y:null}function _(R,y){const T=g(R);return T?T.getPrev(y):null}function B(R,y){const T=g(R);return T?T.getNext(y):null}function N(R){const y=g(R);return y?y.getParent():null}function K(R){const y=g(R);return y?y.getChild():null}const H={treeNodes:b,treeNodeMap:n,levelTreeNodeMap:r,maxLevel:Math.max(...r.keys()),getChildren:h,getFlattenedNodes(R){return Yr(b,R)},getNode:O,getPrev:_,getNext:B,getParent:N,getChild:K,getFirstAvailableNode(){return Vr(b)},getPath(R,y={}){return jr(R,y,H)},getCheckedKeys(R,y={}){const{cascade:T=!0,leafOnly:M=!1,checkStrategy:Y="all",allowNotLoaded:te=!1}=y;return Tt({checkedKeys:Pt(R),indeterminateKeys:kt(R),cascade:T,leafOnly:M,checkStrategy:Y,allowNotLoaded:te},H)},check(R,y,T={}){const{cascade:M=!0,leafOnly:Y=!1,checkStrategy:te="all",allowNotLoaded:J=!1}=T;return Tt({checkedKeys:Pt(y),indeterminateKeys:kt(y),keysToCheck:R==null?[]:fo(R),cascade:M,leafOnly:Y,checkStrategy:te,allowNotLoaded:J},H)},uncheck(R,y,T={}){const{cascade:M=!0,leafOnly:Y=!1,checkStrategy:te="all",allowNotLoaded:J=!1}=T;return Tt({checkedKeys:Pt(y),indeterminateKeys:kt(y),keysToUncheck:R==null?[]:fo(R),cascade:M,leafOnly:Y,checkStrategy:te,allowNotLoaded:J},H)},getNonLeafKeys(R={}){return Pr(b,R)}};return H}const vo=re({name:"NBaseSelectGroupHeader",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(){const{renderLabelRef:e,renderOptionRef:t,labelFieldRef:o,nodePropsRef:n}=ue(Yt);return{labelField:o,nodeProps:n,renderLabel:e,renderOption:t}},render(){const{clsPrefix:e,renderLabel:t,renderOption:o,nodeProps:n,tmNode:{rawNode:r}}=this,i=n==null?void 0:n(r),l=t?t(r,!1):he(r[this.labelField],r,!1),a=d("div",Object.assign({},i,{class:[`${e}-base-select-group-header`,i==null?void 0:i.class]}),l);return r.render?r.render({node:a,option:r}):o?o({node:a,option:r,selected:!1}):a}});function Zr(e,t){return d(Vt,{name:"fade-in-scale-up-transition"},{default:()=>e?d(jt,{clsPrefix:t,class:`${t}-base-select-option__check`},{default:()=>d(Rr)}):null})}const po=re({name:"NBaseSelectOption",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(e){const{valueRef:t,pendingTmNodeRef:o,multipleRef:n,valueSetRef:r,renderLabelRef:i,renderOptionRef:l,labelFieldRef:a,valueFieldRef:s,showCheckmarkRef:c,nodePropsRef:h,handleOptionClick:p,handleOptionMouseEnter:b}=ue(Yt),O=be(()=>{const{value:N}=o;return N?e.tmNode.key===N.key:!1});function g(N){const{tmNode:K}=e;K.disabled||p(N,K)}function _(N){const{tmNode:K}=e;K.disabled||b(N,K)}function B(N){const{tmNode:K}=e,{value:H}=O;K.disabled||H||b(N,K)}return{multiple:n,isGrouped:be(()=>{const{tmNode:N}=e,{parent:K}=N;return K&&K.rawNode.type==="group"}),showCheckmark:c,nodeProps:h,isPending:O,isSelected:be(()=>{const{value:N}=t,{value:K}=n;if(N===null)return!1;const H=e.tmNode.rawNode[s.value];if(K){const{value:R}=r;return R.has(H)}else return N===H}),labelField:a,renderLabel:i,renderOption:l,handleMouseMove:B,handleMouseEnter:_,handleClick:g}},render(){const{clsPrefix:e,tmNode:{rawNode:t},isSelected:o,isPending:n,isGrouped:r,showCheckmark:i,nodeProps:l,renderOption:a,renderLabel:s,handleClick:c,handleMouseEnter:h,handleMouseMove:p}=this,b=Zr(o,e),O=s?[s(t,o),i&&b]:[he(t[this.labelField],t,o),i&&b],g=l==null?void 0:l(t),_=d("div",Object.assign({},g,{class:[`${e}-base-select-option`,t.class,g==null?void 0:g.class,{[`${e}-base-select-option--disabled`]:t.disabled,[`${e}-base-select-option--selected`]:o,[`${e}-base-select-option--grouped`]:r,[`${e}-base-select-option--pending`]:n,[`${e}-base-select-option--show-checkmark`]:i}],style:[(g==null?void 0:g.style)||"",t.style||""],onClick:It([c,g==null?void 0:g.onClick]),onMouseenter:It([h,g==null?void 0:g.onMouseenter]),onMousemove:It([p,g==null?void 0:g.onMousemove])}),d("div",{class:`${e}-base-select-option__content`},O));return t.render?t.render({node:_,option:t,selected:o}):a?a({node:_,option:t,selected:o}):_}}),Jr=S("base-select-menu",`
 line-height: 1.5;
 outline: none;
 z-index: 0;
 position: relative;
 border-radius: var(--n-border-radius);
 transition:
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
 background-color: var(--n-color);
`,[S("scrollbar",`
 max-height: var(--n-height);
 `),S("virtual-list",`
 max-height: var(--n-height);
 `),S("base-select-option",`
 min-height: var(--n-option-height);
 font-size: var(--n-option-font-size);
 display: flex;
 align-items: center;
 `,[k("content",`
 z-index: 1;
 white-space: nowrap;
 text-overflow: ellipsis;
 overflow: hidden;
 `)]),S("base-select-group-header",`
 min-height: var(--n-option-height);
 font-size: .93em;
 display: flex;
 align-items: center;
 `),S("base-select-menu-option-wrapper",`
 position: relative;
 width: 100%;
 `),k("loading, empty",`
 display: flex;
 padding: 12px 32px;
 flex: 1;
 justify-content: center;
 `),k("loading",`
 color: var(--n-loading-color);
 font-size: var(--n-loading-size);
 `),k("header",`
 padding: 8px var(--n-option-padding-left);
 font-size: var(--n-option-font-size);
 transition: 
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 border-bottom: 1px solid var(--n-action-divider-color);
 color: var(--n-action-text-color);
 `),k("action",`
 padding: 8px var(--n-option-padding-left);
 font-size: var(--n-option-font-size);
 transition: 
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 border-top: 1px solid var(--n-action-divider-color);
 color: var(--n-action-text-color);
 `),S("base-select-group-header",`
 position: relative;
 cursor: default;
 padding: var(--n-option-padding);
 color: var(--n-group-header-text-color);
 `),S("base-select-option",`
 cursor: pointer;
 position: relative;
 padding: var(--n-option-padding);
 transition:
 color .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 box-sizing: border-box;
 color: var(--n-option-text-color);
 opacity: 1;
 `,[W("show-checkmark",`
 padding-right: calc(var(--n-option-padding-right) + 20px);
 `),G("&::before",`
 content: "";
 position: absolute;
 left: 4px;
 right: 4px;
 top: 0;
 bottom: 0;
 border-radius: var(--n-border-radius);
 transition: background-color .3s var(--n-bezier);
 `),G("&:active",`
 color: var(--n-option-text-color-pressed);
 `),W("grouped",`
 padding-left: calc(var(--n-option-padding-left) * 1.5);
 `),W("pending",[G("&::before",`
 background-color: var(--n-option-color-pending);
 `)]),W("selected",`
 color: var(--n-option-text-color-active);
 `,[G("&::before",`
 background-color: var(--n-option-color-active);
 `),W("pending",[G("&::before",`
 background-color: var(--n-option-color-active-pending);
 `)])]),W("disabled",`
 cursor: not-allowed;
 `,[Se("selected",`
 color: var(--n-option-text-color-disabled);
 `),W("selected",`
 opacity: var(--n-option-opacity-disabled);
 `)]),k("check",`
 font-size: 16px;
 position: absolute;
 right: calc(var(--n-option-padding-right) - 4px);
 top: calc(50% - 7px);
 color: var(--n-option-check-color);
 transition: color .3s var(--n-bezier);
 `,[Wt({enterScale:"0.5"})])])]),Qr=re({name:"InternalSelectMenu",props:Object.assign(Object.assign({},fe.props),{clsPrefix:{type:String,required:!0},scrollable:{type:Boolean,default:!0},treeMate:{type:Object,required:!0},multiple:Boolean,size:{type:String,default:"medium"},value:{type:[String,Number,Array],default:null},autoPending:Boolean,virtualScroll:{type:Boolean,default:!0},show:{type:Boolean,default:!0},labelField:{type:String,default:"label"},valueField:{type:String,default:"value"},loading:Boolean,focusable:Boolean,renderLabel:Function,renderOption:Function,nodeProps:Function,showCheckmark:{type:Boolean,default:!0},onMousedown:Function,onScroll:Function,onFocus:Function,onBlur:Function,onKeyup:Function,onKeydown:Function,onTabOut:Function,onMouseenter:Function,onMouseleave:Function,onResize:Function,resetMenuOnOptionsChange:{type:Boolean,default:!0},inlineThemeDisabled:Boolean,scrollbarProps:Object,onToggle:Function}),setup(e){const{mergedClsPrefixRef:t,mergedRtlRef:o,mergedComponentPropsRef:n}=Ie(e),r=xo("InternalSelectMenu",o,t),i=fe("InternalSelectMenu","-internal-select-menu",Jr,On,e,ie(e,"clsPrefix")),l=j(null),a=j(null),s=j(null),c=I(()=>e.treeMate.getFlattenedNodes()),h=I(()=>Er(c.value)),p=j(null);function b(){const{treeMate:f}=e;let w=null;const{value:le}=e;le===null?w=f.getFirstAvailableNode():(e.multiple?w=f.getNode((le||[])[(le||[]).length-1]):w=f.getNode(le),(!w||w.disabled)&&(w=f.getFirstAvailableNode())),L(w||null)}function O(){const{value:f}=p;f&&!e.treeMate.getNode(f.key)&&(p.value=null)}let g;Re(()=>e.show,f=>{f?g=Re(()=>e.treeMate,()=>{e.resetMenuOnOptionsChange?(e.autoPending?b():O(),Dt(P)):O()},{immediate:!0}):g==null||g()},{immediate:!0}),Ht(()=>{g==null||g()});const _=I(()=>_t(i.value.self[ge("optionHeight",e.size)])),B=I(()=>Ye(i.value.self[ge("padding",e.size)])),N=I(()=>e.multiple&&Array.isArray(e.value)?new Set(e.value):new Set),K=I(()=>{const f=c.value;return f&&f.length===0}),H=I(()=>{var f,w;return(w=(f=n==null?void 0:n.value)===null||f===void 0?void 0:f.Select)===null||w===void 0?void 0:w.renderEmpty});function R(f){const{onToggle:w}=e;w&&w(f)}function y(f){const{onScroll:w}=e;w&&w(f)}function T(f){var w;(w=s.value)===null||w===void 0||w.sync(),y(f)}function M(){var f;(f=s.value)===null||f===void 0||f.sync()}function Y(){const{value:f}=p;return f||null}function te(f,w){w.disabled||L(w,!1)}function J(f,w){w.disabled||R(w)}function Z(f){var w;Me(f,"action")||(w=e.onKeyup)===null||w===void 0||w.call(e,f)}function se(f){var w;Me(f,"action")||(w=e.onKeydown)===null||w===void 0||w.call(e,f)}function C(f){var w;(w=e.onMousedown)===null||w===void 0||w.call(e,f),!e.focusable&&f.preventDefault()}function x(){const{value:f}=p;f&&L(f.getNext({loop:!0}),!0)}function $(){const{value:f}=p;f&&L(f.getPrev({loop:!0}),!0)}function L(f,w=!1){p.value=f,w&&P()}function P(){var f,w;const le=p.value;if(!le)return;const me=h.value(le.key);me!==null&&(e.virtualScroll?(f=a.value)===null||f===void 0||f.scrollTo({index:me}):(w=s.value)===null||w===void 0||w.scrollTo({index:me,elSize:_.value}))}function V(f){var w,le;!((w=l.value)===null||w===void 0)&&w.contains(f.target)&&((le=e.onFocus)===null||le===void 0||le.call(e,f))}function q(f){var w,le;!((w=l.value)===null||w===void 0)&&w.contains(f.relatedTarget)||(le=e.onBlur)===null||le===void 0||le.call(e,f)}pe(Yt,{handleOptionMouseEnter:te,handleOptionClick:J,valueSetRef:N,pendingTmNodeRef:p,nodePropsRef:ie(e,"nodeProps"),showCheckmarkRef:ie(e,"showCheckmark"),multipleRef:ie(e,"multiple"),valueRef:ie(e,"value"),renderLabelRef:ie(e,"renderLabel"),renderOptionRef:ie(e,"renderOption"),labelFieldRef:ie(e,"labelField"),valueFieldRef:ie(e,"valueField")}),pe(ar,l),He(()=>{const{value:f}=s;f&&f.sync()});const ne=I(()=>{const{size:f}=e,{common:{cubicBezierEaseInOut:w},self:{height:le,borderRadius:me,color:we,groupHeaderTextColor:m,actionDividerColor:ye,optionTextColorPressed:Be,optionTextColor:ke,optionTextColorDisabled:De,optionTextColorActive:je,optionOpacityDisabled:Ve,optionCheckColor:Ne,actionTextColor:Fe,optionColorPending:We,optionColorActive:Ue,loadingColor:Ge,loadingSize:$e,optionColorActivePending:Ee,[ge("optionFontSize",f)]:xe,[ge("optionHeight",f)]:z,[ge("optionPadding",f)]:E}}=i.value;return{"--n-height":le,"--n-action-divider-color":ye,"--n-action-text-color":Fe,"--n-bezier":w,"--n-border-radius":me,"--n-color":we,"--n-option-font-size":xe,"--n-group-header-text-color":m,"--n-option-check-color":Ne,"--n-option-color-pending":We,"--n-option-color-active":Ue,"--n-option-color-active-pending":Ee,"--n-option-height":z,"--n-option-opacity-disabled":Ve,"--n-option-text-color":ke,"--n-option-text-color-active":je,"--n-option-text-color-disabled":De,"--n-option-text-color-pressed":Be,"--n-option-padding":E,"--n-option-padding-left":Ye(E,"left"),"--n-option-padding-right":Ye(E,"right"),"--n-loading-color":Ge,"--n-loading-size":$e}}),{inlineThemeDisabled:A}=e,D=A?Pe("internal-select-menu",I(()=>e.size[0]),ne,e):void 0,v={selfRef:l,next:x,prev:$,getPendingTmNode:Y};return _o(l,e.onResize),Object.assign({mergedTheme:i,mergedClsPrefix:t,rtlEnabled:r,virtualListRef:a,scrollbarRef:s,itemSize:_,padding:B,flattenedNodes:c,empty:K,mergedRenderEmpty:H,virtualListContainer(){const{value:f}=a;return f==null?void 0:f.listElRef},virtualListContent(){const{value:f}=a;return f==null?void 0:f.itemsElRef},doScroll:y,handleFocusin:V,handleFocusout:q,handleKeyUp:Z,handleKeyDown:se,handleMouseDown:C,handleVirtualListResize:M,handleVirtualListScroll:T,cssVars:A?void 0:ne,themeClass:D==null?void 0:D.themeClass,onRender:D==null?void 0:D.onRender},v)},render(){const{$slots:e,virtualScroll:t,clsPrefix:o,mergedTheme:n,themeClass:r,onRender:i}=this;return i==null||i(),d("div",{ref:"selfRef",tabindex:this.focusable?0:-1,class:[`${o}-base-select-menu`,`${o}-base-select-menu--${this.size}-size`,this.rtlEnabled&&`${o}-base-select-menu--rtl`,r,this.multiple&&`${o}-base-select-menu--multiple`],style:this.cssVars,onFocusin:this.handleFocusin,onFocusout:this.handleFocusout,onKeyup:this.handleKeyUp,onKeydown:this.handleKeyDown,onMousedown:this.handleMouseDown,onMouseenter:this.onMouseenter,onMouseleave:this.onMouseleave},ro(e.header,l=>l&&d("div",{class:`${o}-base-select-menu__header`,"data-header":!0,key:"header"},l)),this.loading?d("div",{class:`${o}-base-select-menu__loading`},d(kn,{clsPrefix:o,strokeWidth:20})):this.empty?d("div",{class:`${o}-base-select-menu__empty`,"data-empty":!0},Tn(e.empty,()=>{var l;return[((l=this.mergedRenderEmpty)===null||l===void 0?void 0:l.call(this))||d(lr,{theme:n.peers.Empty,themeOverrides:n.peerOverrides.Empty,size:this.size})]})):d(Ut,Object.assign({ref:"scrollbarRef",theme:n.peers.Scrollbar,themeOverrides:n.peerOverrides.Scrollbar,scrollable:this.scrollable,container:t?this.virtualListContainer:void 0,content:t?this.virtualListContent:void 0,onScroll:t?void 0:this.doScroll},this.scrollbarProps),{default:()=>t?d(wr,{ref:"virtualListRef",class:`${o}-virtual-list`,items:this.flattenedNodes,itemSize:this.itemSize,showScrollbar:!1,paddingTop:this.padding.top,paddingBottom:this.padding.bottom,onResize:this.handleVirtualListResize,onScroll:this.handleVirtualListScroll,itemResizable:!0},{default:({item:l})=>l.isGroup?d(vo,{key:l.key,clsPrefix:o,tmNode:l}):l.ignored?null:d(po,{clsPrefix:o,key:l.key,tmNode:l})}):d("div",{class:`${o}-base-select-menu-option-wrapper`,style:{paddingTop:this.padding.top,paddingBottom:this.padding.bottom}},this.flattenedNodes.map(l=>l.isGroup?d(vo,{key:l.key,clsPrefix:o,tmNode:l}):d(po,{clsPrefix:o,key:l.key,tmNode:l})))}),ro(e.action,l=>l&&[d("div",{class:`${o}-base-select-menu__action`,"data-action":!0,key:"action"},l),d(Ir,{onFocus:this.onTabOut,key:"focus-detector"})]))}}),ei=G([S("base-selection",`
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
 `,[S("base-loading",`
 color: var(--n-loading-color);
 `),S("base-selection-tags","min-height: var(--n-height);"),k("border, state-border",`
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
 `),k("state-border",`
 z-index: 1;
 border-color: #0000;
 `),S("base-suffix",`
 cursor: pointer;
 position: absolute;
 top: 50%;
 transform: translateY(-50%);
 right: 10px;
 `,[k("arrow",`
 font-size: var(--n-arrow-size);
 color: var(--n-arrow-color);
 transition: color .3s var(--n-bezier);
 `)]),S("base-selection-overlay",`
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
 `,[k("wrapper",`
 flex-basis: 0;
 flex-grow: 1;
 overflow: hidden;
 text-overflow: ellipsis;
 `)]),S("base-selection-placeholder",`
 color: var(--n-placeholder-color);
 `,[k("inner",`
 max-width: 100%;
 overflow: hidden;
 `)]),S("base-selection-tags",`
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
 `),S("base-selection-label",`
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
 `,[S("base-selection-input",`
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
 `,[k("content",`
 text-overflow: ellipsis;
 overflow: hidden;
 white-space: nowrap; 
 `)]),k("render-label",`
 color: var(--n-text-color);
 `)]),Se("disabled",[G("&:hover",[k("state-border",`
 box-shadow: var(--n-box-shadow-hover);
 border: var(--n-border-hover);
 `)]),W("focus",[k("state-border",`
 box-shadow: var(--n-box-shadow-focus);
 border: var(--n-border-focus);
 `)]),W("active",[k("state-border",`
 box-shadow: var(--n-box-shadow-active);
 border: var(--n-border-active);
 `),S("base-selection-label","background-color: var(--n-color-active);"),S("base-selection-tags","background-color: var(--n-color-active);")])]),W("disabled","cursor: not-allowed;",[k("arrow",`
 color: var(--n-arrow-color-disabled);
 `),S("base-selection-label",`
 cursor: not-allowed;
 background-color: var(--n-color-disabled);
 `,[S("base-selection-input",`
 cursor: not-allowed;
 color: var(--n-text-color-disabled);
 `),k("render-label",`
 color: var(--n-text-color-disabled);
 `)]),S("base-selection-tags",`
 cursor: not-allowed;
 background-color: var(--n-color-disabled);
 `),S("base-selection-placeholder",`
 cursor: not-allowed;
 color: var(--n-placeholder-color-disabled);
 `)]),S("base-selection-input-tag",`
 height: calc(var(--n-height) - 6px);
 line-height: calc(var(--n-height) - 6px);
 outline: none;
 display: none;
 position: relative;
 margin-bottom: 3px;
 max-width: 100%;
 vertical-align: bottom;
 `,[k("input",`
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
 `),k("mirror",`
 position: absolute;
 left: 0;
 top: 0;
 white-space: pre;
 visibility: hidden;
 user-select: none;
 -webkit-user-select: none;
 opacity: 0;
 `)]),["warning","error"].map(e=>W(`${e}-status`,[k("state-border",`border: var(--n-border-${e});`),Se("disabled",[G("&:hover",[k("state-border",`
 box-shadow: var(--n-box-shadow-hover-${e});
 border: var(--n-border-hover-${e});
 `)]),W("active",[k("state-border",`
 box-shadow: var(--n-box-shadow-active-${e});
 border: var(--n-border-active-${e});
 `),S("base-selection-label",`background-color: var(--n-color-active-${e});`),S("base-selection-tags",`background-color: var(--n-color-active-${e});`)]),W("focus",[k("state-border",`
 box-shadow: var(--n-box-shadow-focus-${e});
 border: var(--n-border-focus-${e});
 `)])])]))]),S("base-selection-popover",`
 margin-bottom: -3px;
 display: flex;
 flex-wrap: wrap;
 margin-right: -8px;
 `),S("base-selection-tag-wrapper",`
 max-width: 100%;
 display: inline-flex;
 padding: 0 7px 3px 0;
 `,[G("&:last-child","padding-right: 0;"),S("tag",`
 font-size: 14px;
 max-width: 100%;
 `,[k("content",`
 line-height: 1.25;
 text-overflow: ellipsis;
 overflow: hidden;
 `)])])]),ti=re({name:"InternalSelection",props:Object.assign(Object.assign({},fe.props),{clsPrefix:{type:String,required:!0},bordered:{type:Boolean,default:void 0},active:Boolean,pattern:{type:String,default:""},placeholder:String,selectedOption:{type:Object,default:null},selectedOptions:{type:Array,default:null},labelField:{type:String,default:"label"},valueField:{type:String,default:"value"},multiple:Boolean,filterable:Boolean,clearable:Boolean,disabled:Boolean,size:{type:String,default:"medium"},loading:Boolean,autofocus:Boolean,showArrow:{type:Boolean,default:!0},inputProps:Object,focused:Boolean,renderTag:Function,onKeydown:Function,onClick:Function,onBlur:Function,onFocus:Function,onDeleteOption:Function,maxTagCount:[String,Number],ellipsisTagPopoverProps:Object,onClear:Function,onPatternInput:Function,onPatternFocus:Function,onPatternBlur:Function,renderLabel:Function,status:String,inlineThemeDisabled:Boolean,ignoreComposition:{type:Boolean,default:!0},onResize:Function}),setup(e){const{mergedClsPrefixRef:t,mergedRtlRef:o}=Ie(e),n=xo("InternalSelection",o,t),r=j(null),i=j(null),l=j(null),a=j(null),s=j(null),c=j(null),h=j(null),p=j(null),b=j(null),O=j(null),g=j(!1),_=j(!1),B=j(!1),N=fe("InternalSelection","-internal-selection",ei,Fn,e,ie(e,"clsPrefix")),K=I(()=>e.clearable&&!e.disabled&&(B.value||e.active)),H=I(()=>e.selectedOption?e.renderTag?e.renderTag({option:e.selectedOption,handleClose:()=>{}}):e.renderLabel?e.renderLabel(e.selectedOption,!0):he(e.selectedOption[e.labelField],e.selectedOption,!0):e.placeholder),R=I(()=>{const z=e.selectedOption;if(z)return z[e.labelField]}),y=I(()=>e.multiple?!!(Array.isArray(e.selectedOptions)&&e.selectedOptions.length):e.selectedOption!==null);function T(){var z;const{value:E}=r;if(E){const{value:de}=i;de&&(de.style.width=`${E.offsetWidth}px`,e.maxTagCount!=="responsive"&&((z=b.value)===null||z===void 0||z.sync({showAllItemsBeforeCalculate:!1})))}}function M(){const{value:z}=O;z&&(z.style.display="none")}function Y(){const{value:z}=O;z&&(z.style.display="inline-block")}Re(ie(e,"active"),z=>{z||M()}),Re(ie(e,"pattern"),()=>{e.multiple&&Dt(T)});function te(z){const{onFocus:E}=e;E&&E(z)}function J(z){const{onBlur:E}=e;E&&E(z)}function Z(z){const{onDeleteOption:E}=e;E&&E(z)}function se(z){const{onClear:E}=e;E&&E(z)}function C(z){const{onPatternInput:E}=e;E&&E(z)}function x(z){var E;(!z.relatedTarget||!(!((E=l.value)===null||E===void 0)&&E.contains(z.relatedTarget)))&&te(z)}function $(z){var E;!((E=l.value)===null||E===void 0)&&E.contains(z.relatedTarget)||J(z)}function L(z){se(z)}function P(){B.value=!0}function V(){B.value=!1}function q(z){!e.active||!e.filterable||z.target!==i.value&&z.preventDefault()}function ne(z){Z(z)}const A=j(!1);function D(z){if(z.key==="Backspace"&&!A.value&&!e.pattern.length){const{selectedOptions:E}=e;E!=null&&E.length&&ne(E[E.length-1])}}let v=null;function f(z){const{value:E}=r;if(E){const de=z.target.value;E.textContent=de,T()}e.ignoreComposition&&A.value?v=z:C(z)}function w(){A.value=!0}function le(){A.value=!1,e.ignoreComposition&&C(v),v=null}function me(z){var E;_.value=!0,(E=e.onPatternFocus)===null||E===void 0||E.call(e,z)}function we(z){var E;_.value=!1,(E=e.onPatternBlur)===null||E===void 0||E.call(e,z)}function m(){var z,E;if(e.filterable)_.value=!1,(z=c.value)===null||z===void 0||z.blur(),(E=i.value)===null||E===void 0||E.blur();else if(e.multiple){const{value:de}=a;de==null||de.blur()}else{const{value:de}=s;de==null||de.blur()}}function ye(){var z,E,de;e.filterable?(_.value=!1,(z=c.value)===null||z===void 0||z.focus()):e.multiple?(E=a.value)===null||E===void 0||E.focus():(de=s.value)===null||de===void 0||de.focus()}function Be(){const{value:z}=i;z&&(Y(),z.focus())}function ke(){const{value:z}=i;z&&z.blur()}function De(z){const{value:E}=h;E&&E.setTextContent(`+${z}`)}function je(){const{value:z}=p;return z}function Ve(){return i.value}let Ne=null;function Fe(){Ne!==null&&window.clearTimeout(Ne)}function We(){e.active||(Fe(),Ne=window.setTimeout(()=>{y.value&&(g.value=!0)},100))}function Ue(){Fe()}function Ge(z){z||(Fe(),g.value=!1)}Re(y,z=>{z||(g.value=!1)}),He(()=>{At(()=>{const z=c.value;z&&(e.disabled?z.removeAttribute("tabindex"):z.tabIndex=_.value?-1:0)})}),_o(l,e.onResize);const{inlineThemeDisabled:$e}=e,Ee=I(()=>{const{size:z}=e,{common:{cubicBezierEaseInOut:E},self:{fontWeight:de,borderRadius:pt,color:mt,placeholderColor:gt,textColor:Qe,paddingSingle:et,paddingMultiple:tt,caretColor:bt,colorDisabled:yt,textColorDisabled:ot,placeholderColorDisabled:Te,colorActive:u,boxShadowFocus:F,boxShadowActive:U,boxShadowHover:ee,border:X,borderFocus:Q,borderHover:oe,borderActive:ce,arrowColor:Ce,arrowColorDisabled:Yo,loadingColor:Xo,colorActiveWarning:Zo,boxShadowFocusWarning:Jo,boxShadowActiveWarning:Qo,boxShadowHoverWarning:en,borderWarning:tn,borderFocusWarning:on,borderHoverWarning:nn,borderActiveWarning:rn,colorActiveError:ln,boxShadowFocusError:an,boxShadowActiveError:sn,boxShadowHoverError:dn,borderError:cn,borderFocusError:un,borderHoverError:fn,borderActiveError:hn,clearColor:vn,clearColorHover:pn,clearColorPressed:mn,clearSize:gn,arrowSize:bn,[ge("height",z)]:yn,[ge("fontSize",z)]:xn}}=N.value,nt=Ye(et),rt=Ye(tt);return{"--n-bezier":E,"--n-border":X,"--n-border-active":ce,"--n-border-focus":Q,"--n-border-hover":oe,"--n-border-radius":pt,"--n-box-shadow-active":U,"--n-box-shadow-focus":F,"--n-box-shadow-hover":ee,"--n-caret-color":bt,"--n-color":mt,"--n-color-active":u,"--n-color-disabled":yt,"--n-font-size":xn,"--n-height":yn,"--n-padding-single-top":nt.top,"--n-padding-multiple-top":rt.top,"--n-padding-single-right":nt.right,"--n-padding-multiple-right":rt.right,"--n-padding-single-left":nt.left,"--n-padding-multiple-left":rt.left,"--n-padding-single-bottom":nt.bottom,"--n-padding-multiple-bottom":rt.bottom,"--n-placeholder-color":gt,"--n-placeholder-color-disabled":Te,"--n-text-color":Qe,"--n-text-color-disabled":ot,"--n-arrow-color":Ce,"--n-arrow-color-disabled":Yo,"--n-loading-color":Xo,"--n-color-active-warning":Zo,"--n-box-shadow-focus-warning":Jo,"--n-box-shadow-active-warning":Qo,"--n-box-shadow-hover-warning":en,"--n-border-warning":tn,"--n-border-focus-warning":on,"--n-border-hover-warning":nn,"--n-border-active-warning":rn,"--n-color-active-error":ln,"--n-box-shadow-focus-error":an,"--n-box-shadow-active-error":sn,"--n-box-shadow-hover-error":dn,"--n-border-error":cn,"--n-border-focus-error":un,"--n-border-hover-error":fn,"--n-border-active-error":hn,"--n-clear-size":gn,"--n-clear-color":vn,"--n-clear-color-hover":pn,"--n-clear-color-pressed":mn,"--n-arrow-size":bn,"--n-font-weight":de}}),xe=$e?Pe("internal-selection",I(()=>e.size[0]),Ee,e):void 0;return{mergedTheme:N,mergedClearable:K,mergedClsPrefix:t,rtlEnabled:n,patternInputFocused:_,filterablePlaceholder:H,label:R,selected:y,showTagsPanel:g,isComposing:A,counterRef:h,counterWrapperRef:p,patternInputMirrorRef:r,patternInputRef:i,selfRef:l,multipleElRef:a,singleElRef:s,patternInputWrapperRef:c,overflowRef:b,inputTagElRef:O,handleMouseDown:q,handleFocusin:x,handleClear:L,handleMouseEnter:P,handleMouseLeave:V,handleDeleteOption:ne,handlePatternKeyDown:D,handlePatternInputInput:f,handlePatternInputBlur:we,handlePatternInputFocus:me,handleMouseEnterCounter:We,handleMouseLeaveCounter:Ue,handleFocusout:$,handleCompositionEnd:le,handleCompositionStart:w,onPopoverUpdateShow:Ge,focus:ye,focusInput:Be,blur:m,blurInput:ke,updateCounter:De,getCounter:je,getTail:Ve,renderLabel:e.renderLabel,cssVars:$e?void 0:Ee,themeClass:xe==null?void 0:xe.themeClass,onRender:xe==null?void 0:xe.onRender}},render(){const{status:e,multiple:t,size:o,disabled:n,filterable:r,maxTagCount:i,bordered:l,clsPrefix:a,ellipsisTagPopoverProps:s,onRender:c,renderTag:h,renderLabel:p}=this;c==null||c();const b=i==="responsive",O=typeof i=="number",g=b||O,_=d(Nn,null,{default:()=>d(fr,{clsPrefix:a,loading:this.loading,showArrow:this.showArrow,showClear:this.mergedClearable&&this.selected,onClear:this.handleClear},{default:()=>{var N,K;return(K=(N=this.$slots).arrow)===null||K===void 0?void 0:K.call(N)}})});let B;if(t){const{labelField:N}=this,K=C=>d("div",{class:`${a}-base-selection-tag-wrapper`,key:C.value},h?h({option:C,handleClose:()=>{this.handleDeleteOption(C)}}):d(Rt,{size:o,closable:!C.disabled,disabled:n,onClose:()=>{this.handleDeleteOption(C)},internalCloseIsButtonTag:!1,internalCloseFocusable:!1},{default:()=>p?p(C,!0):he(C[N],C,!0)})),H=()=>(O?this.selectedOptions.slice(0,i):this.selectedOptions).map(K),R=r?d("div",{class:`${a}-base-selection-input-tag`,ref:"inputTagElRef",key:"__input-tag__"},d("input",Object.assign({},this.inputProps,{ref:"patternInputRef",tabindex:-1,disabled:n,value:this.pattern,autofocus:this.autofocus,class:`${a}-base-selection-input-tag__input`,onBlur:this.handlePatternInputBlur,onFocus:this.handlePatternInputFocus,onKeydown:this.handlePatternKeyDown,onInput:this.handlePatternInputInput,onCompositionstart:this.handleCompositionStart,onCompositionend:this.handleCompositionEnd})),d("span",{ref:"patternInputMirrorRef",class:`${a}-base-selection-input-tag__mirror`},this.pattern)):null,y=b?()=>d("div",{class:`${a}-base-selection-tag-wrapper`,ref:"counterWrapperRef"},d(Rt,{size:o,ref:"counterRef",onMouseenter:this.handleMouseEnterCounter,onMouseleave:this.handleMouseLeaveCounter,disabled:n})):void 0;let T;if(O){const C=this.selectedOptions.length-i;C>0&&(T=d("div",{class:`${a}-base-selection-tag-wrapper`,key:"__counter__"},d(Rt,{size:o,ref:"counterRef",onMouseenter:this.handleMouseEnterCounter,disabled:n},{default:()=>`+${C}`})))}const M=b?r?d(Bt,{ref:"overflowRef",updateCounter:this.updateCounter,getCounter:this.getCounter,getTail:this.getTail,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:H,counter:y,tail:()=>R}):d(Bt,{ref:"overflowRef",updateCounter:this.updateCounter,getCounter:this.getCounter,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:H,counter:y}):O&&T?H().concat(T):H(),Y=g?()=>d("div",{class:`${a}-base-selection-popover`},b?H():this.selectedOptions.map(K)):void 0,te=g?Object.assign({show:this.showTagsPanel,trigger:"hover",overlap:!0,placement:"top",width:"trigger",onUpdateShow:this.onPopoverUpdateShow,theme:this.mergedTheme.peers.Popover,themeOverrides:this.mergedTheme.peerOverrides.Popover},s):null,Z=(this.selected?!1:this.active?!this.pattern&&!this.isComposing:!0)?d("div",{class:`${a}-base-selection-placeholder ${a}-base-selection-overlay`},d("div",{class:`${a}-base-selection-placeholder__inner`},this.placeholder)):null,se=r?d("div",{ref:"patternInputWrapperRef",class:`${a}-base-selection-tags`},M,b?null:R,_):d("div",{ref:"multipleElRef",class:`${a}-base-selection-tags`,tabindex:n?void 0:0},M,_);B=d(Gt,null,g?d(zo,Object.assign({},te,{scrollable:!0,style:"max-height: calc(var(--v-target-height) * 6.6);"}),{trigger:()=>se,default:Y}):se,Z)}else if(r){const N=this.pattern||this.isComposing,K=this.active?!N:!this.selected,H=this.active?!1:this.selected;B=d("div",{ref:"patternInputWrapperRef",class:`${a}-base-selection-label`,title:this.patternInputFocused?void 0:uo(this.label)},d("input",Object.assign({},this.inputProps,{ref:"patternInputRef",class:`${a}-base-selection-input`,value:this.active?this.pattern:"",placeholder:"",readonly:n,disabled:n,tabindex:-1,autofocus:this.autofocus,onFocus:this.handlePatternInputFocus,onBlur:this.handlePatternInputBlur,onInput:this.handlePatternInputInput,onCompositionstart:this.handleCompositionStart,onCompositionend:this.handleCompositionEnd})),H?d("div",{class:`${a}-base-selection-label__render-label ${a}-base-selection-overlay`,key:"input"},d("div",{class:`${a}-base-selection-overlay__wrapper`},h?h({option:this.selectedOption,handleClose:()=>{}}):p?p(this.selectedOption,!0):he(this.label,this.selectedOption,!0))):null,K?d("div",{class:`${a}-base-selection-placeholder ${a}-base-selection-overlay`,key:"placeholder"},d("div",{class:`${a}-base-selection-overlay__wrapper`},this.filterablePlaceholder)):null,_)}else B=d("div",{ref:"singleElRef",class:`${a}-base-selection-label`,tabindex:this.disabled?void 0:0},this.label!==void 0?d("div",{class:`${a}-base-selection-input`,title:uo(this.label),key:"input"},d("div",{class:`${a}-base-selection-input__content`},h?h({option:this.selectedOption,handleClose:()=>{}}):p?p(this.selectedOption,!0):he(this.label,this.selectedOption,!0))):d("div",{class:`${a}-base-selection-placeholder ${a}-base-selection-overlay`,key:"placeholder"},d("div",{class:`${a}-base-selection-placeholder__inner`},this.placeholder)),_);return d("div",{ref:"selfRef",class:[`${a}-base-selection`,this.rtlEnabled&&`${a}-base-selection--rtl`,this.themeClass,e&&`${a}-base-selection--${e}-status`,{[`${a}-base-selection--active`]:this.active,[`${a}-base-selection--selected`]:this.selected||this.active&&this.pattern,[`${a}-base-selection--disabled`]:this.disabled,[`${a}-base-selection--multiple`]:this.multiple,[`${a}-base-selection--focus`]:this.focused}],style:this.cssVars,onClick:this.onClick,onMouseenter:this.handleMouseEnter,onMouseleave:this.handleMouseLeave,onKeydown:this.onKeydown,onFocusin:this.handleFocusin,onFocusout:this.handleFocusout,onMousedown:this.handleMouseDown},B,l?d("div",{class:`${a}-base-selection__border`}):null,l?d("div",{class:`${a}-base-selection__state-border`}):null)}});function ht(e){return e.type==="group"}function $o(e){return e.type==="ignored"}function Ot(e,t){try{return!!(1+t.toString().toLowerCase().indexOf(e.trim().toLowerCase()))}catch{return!1}}function oi(e,t){return{getIsGroup:ht,getIgnored:$o,getKey(n){return ht(n)?n.name||n.key||"key-required":n[e]},getChildren(n){return n[t]}}}function ni(e,t,o,n){if(!t)return e;function r(i){if(!Array.isArray(i))return[];const l=[];for(const a of i)if(ht(a)){const s=r(a[n]);s.length&&l.push(Object.assign({},a,{[n]:s}))}else{if($o(a))continue;t(o,a)&&l.push(a)}return l}return r(e)}function ri(e,t,o){const n=new Map;return e.forEach(r=>{ht(r)?r[o].forEach(i=>{n.set(i[t],i)}):n.set(r[t],r)}),n}const ii=G([S("select",`
 z-index: auto;
 outline: none;
 width: 100%;
 position: relative;
 font-weight: var(--n-font-weight);
 `),S("select-menu",`
 margin: 4px 0;
 box-shadow: var(--n-menu-box-shadow);
 `,[Wt({originalTransition:"background-color .3s var(--n-bezier), box-shadow .3s var(--n-bezier)"})])]),li=Object.assign(Object.assign({},fe.props),{to:Mt.propTo,bordered:{type:Boolean,default:void 0},clearable:Boolean,clearCreatedOptionsOnClear:{type:Boolean,default:!0},clearFilterAfterSelect:{type:Boolean,default:!0},options:{type:Array,default:()=>[]},defaultValue:{type:[String,Number,Array],default:null},keyboard:{type:Boolean,default:!0},value:[String,Number,Array],placeholder:String,menuProps:Object,multiple:Boolean,size:String,menuSize:{type:String},filterable:Boolean,disabled:{type:Boolean,default:void 0},remote:Boolean,loading:Boolean,filter:Function,placement:{type:String,default:"bottom-start"},widthMode:{type:String,default:"trigger"},tag:Boolean,onCreate:Function,fallbackOption:{type:[Function,Boolean],default:void 0},show:{type:Boolean,default:void 0},showArrow:{type:Boolean,default:!0},maxTagCount:[Number,String],ellipsisTagPopoverProps:Object,consistentMenuWidth:{type:Boolean,default:!0},virtualScroll:{type:Boolean,default:!0},labelField:{type:String,default:"label"},valueField:{type:String,default:"value"},childrenField:{type:String,default:"children"},renderLabel:Function,renderOption:Function,renderTag:Function,"onUpdate:value":[Function,Array],inputProps:Object,nodeProps:Function,ignoreComposition:{type:Boolean,default:!0},showOnFocus:Boolean,onUpdateValue:[Function,Array],onBlur:[Function,Array],onClear:[Function,Array],onFocus:[Function,Array],onScroll:[Function,Array],onSearch:[Function,Array],onUpdateShow:[Function,Array],"onUpdate:show":[Function,Array],displayDirective:{type:String,default:"show"},resetMenuOnOptionsChange:{type:Boolean,default:!0},status:String,showCheckmark:{type:Boolean,default:!0},scrollbarProps:Object,onChange:[Function,Array],items:Array}),ai=re({name:"Select",props:li,slots:Object,setup(e){const{mergedClsPrefixRef:t,mergedBorderedRef:o,namespaceRef:n,inlineThemeDisabled:r,mergedComponentPropsRef:i}=Ie(e),l=fe("Select","-select",ii,$n,e,t),a=j(e.defaultValue),s=ie(e,"value"),c=Ke(s,a),h=j(!1),p=j(""),b=To(e,["items","options"]),O=j([]),g=j([]),_=I(()=>g.value.concat(O.value).concat(b.value)),B=I(()=>{const{filter:u}=e;if(u)return u;const{labelField:F,valueField:U}=e;return(ee,X)=>{if(!X)return!1;const Q=X[F];if(typeof Q=="string")return Ot(ee,Q);const oe=X[U];return typeof oe=="string"?Ot(ee,oe):typeof oe=="number"?Ot(ee,String(oe)):!1}}),N=I(()=>{if(e.remote)return b.value;{const{value:u}=_,{value:F}=p;return!F.length||!e.filterable?u:ni(u,B.value,F,e.childrenField)}}),K=I(()=>{const{valueField:u,childrenField:F}=e,U=oi(u,F);return Xe(N.value,U)}),H=I(()=>ri(_.value,e.valueField,e.childrenField)),R=j(!1),y=Ke(ie(e,"show"),R),T=j(null),M=j(null),Y=j(null),{localeRef:te}=hr("Select"),J=I(()=>{var u;return(u=e.placeholder)!==null&&u!==void 0?u:te.value.placeholder}),Z=[],se=j(new Map),C=I(()=>{const{fallbackOption:u}=e;if(u===void 0){const{labelField:F,valueField:U}=e;return ee=>({[F]:String(ee),[U]:ee})}return u===!1?!1:F=>Object.assign(u(F),{value:F})});function x(u){const F=e.remote,{value:U}=se,{value:ee}=H,{value:X}=C,Q=[];return u.forEach(oe=>{if(ee.has(oe))Q.push(ee.get(oe));else if(F&&U.has(oe))Q.push(U.get(oe));else if(X){const ce=X(oe);ce&&Q.push(ce)}}),Q}const $=I(()=>{if(e.multiple){const{value:u}=c;return Array.isArray(u)?x(u):[]}return null}),L=I(()=>{const{value:u}=c;return!e.multiple&&!Array.isArray(u)?u===null?null:x([u])[0]||null:null}),P=Mn(e,{mergedSize:u=>{var F,U;const{size:ee}=e;if(ee)return ee;const{mergedSize:X}=u||{};if(X!=null&&X.value)return X.value;const Q=(U=(F=i==null?void 0:i.value)===null||F===void 0?void 0:F.Select)===null||U===void 0?void 0:U.size;return Q||"medium"}}),{mergedSizeRef:V,mergedDisabledRef:q,mergedStatusRef:ne}=P;function A(u,F){const{onChange:U,"onUpdate:value":ee,onUpdateValue:X}=e,{nTriggerFormChange:Q,nTriggerFormInput:oe}=P;U&&ae(U,u,F),X&&ae(X,u,F),ee&&ae(ee,u,F),a.value=u,Q(),oe()}function D(u){const{onBlur:F}=e,{nTriggerFormBlur:U}=P;F&&ae(F,u),U()}function v(){const{onClear:u}=e;u&&ae(u)}function f(u){const{onFocus:F,showOnFocus:U}=e,{nTriggerFormFocus:ee}=P;F&&ae(F,u),ee(),U&&m()}function w(u){const{onSearch:F}=e;F&&ae(F,u)}function le(u){const{onScroll:F}=e;F&&ae(F,u)}function me(){var u;const{remote:F,multiple:U}=e;if(F){const{value:ee}=se;if(U){const{valueField:X}=e;(u=$.value)===null||u===void 0||u.forEach(Q=>{ee.set(Q[X],Q)})}else{const X=L.value;X&&ee.set(X[e.valueField],X)}}}function we(u){const{onUpdateShow:F,"onUpdate:show":U}=e;F&&ae(F,u),U&&ae(U,u),R.value=u}function m(){q.value||(we(!0),R.value=!0,e.filterable&&tt())}function ye(){we(!1)}function Be(){p.value="",g.value=Z}const ke=j(!1);function De(){e.filterable&&(ke.value=!0)}function je(){e.filterable&&(ke.value=!1,y.value||Be())}function Ve(){q.value||(y.value?e.filterable?tt():ye():m())}function Ne(u){var F,U;!((U=(F=Y.value)===null||F===void 0?void 0:F.selfRef)===null||U===void 0)&&U.contains(u.relatedTarget)||(h.value=!1,D(u),ye())}function Fe(u){f(u),h.value=!0}function We(){h.value=!0}function Ue(u){var F;!((F=T.value)===null||F===void 0)&&F.$el.contains(u.relatedTarget)||(h.value=!1,D(u),ye())}function Ge(){var u;(u=T.value)===null||u===void 0||u.focus(),ye()}function $e(u){var F;y.value&&(!((F=T.value)===null||F===void 0)&&F.$el.contains(En(u))||ye())}function Ee(u){if(!Array.isArray(u))return[];if(C.value)return Array.from(u);{const{remote:F}=e,{value:U}=H;if(F){const{value:ee}=se;return u.filter(X=>U.has(X)||ee.has(X))}else return u.filter(ee=>U.has(ee))}}function xe(u){z(u.rawNode)}function z(u){if(q.value)return;const{tag:F,remote:U,clearFilterAfterSelect:ee,valueField:X}=e;if(F&&!U){const{value:Q}=g,oe=Q[0]||null;if(oe){const ce=O.value;ce.length?ce.push(oe):O.value=[oe],g.value=Z}}if(U&&se.value.set(u[X],u),e.multiple){const Q=Ee(c.value),oe=Q.findIndex(ce=>ce===u[X]);if(~oe){if(Q.splice(oe,1),F&&!U){const ce=E(u[X]);~ce&&(O.value.splice(ce,1),ee&&(p.value=""))}}else Q.push(u[X]),ee&&(p.value="");A(Q,x(Q))}else{if(F&&!U){const Q=E(u[X]);~Q?O.value=[O.value[Q]]:O.value=Z}et(),ye(),A(u[X],u)}}function E(u){return O.value.findIndex(U=>U[e.valueField]===u)}function de(u){y.value||m();const{value:F}=u.target;p.value=F;const{tag:U,remote:ee}=e;if(w(F),U&&!ee){if(!F){g.value=Z;return}const{onCreate:X}=e,Q=X?X(F):{[e.labelField]:F,[e.valueField]:F},{valueField:oe,labelField:ce}=e;b.value.some(Ce=>Ce[oe]===Q[oe]||Ce[ce]===Q[ce])||O.value.some(Ce=>Ce[oe]===Q[oe]||Ce[ce]===Q[ce])?g.value=Z:g.value=[Q]}}function pt(u){u.stopPropagation();const{multiple:F,tag:U,remote:ee,clearCreatedOptionsOnClear:X}=e;!F&&e.filterable&&ye(),U&&!ee&&X&&(O.value=Z),v(),F?A([],[]):A(null,null)}function mt(u){!Me(u,"action")&&!Me(u,"empty")&&!Me(u,"header")&&u.preventDefault()}function gt(u){le(u)}function Qe(u){var F,U,ee,X,Q;if(!e.keyboard){u.preventDefault();return}switch(u.key){case" ":if(e.filterable)break;u.preventDefault();case"Enter":if(!(!((F=T.value)===null||F===void 0)&&F.isComposing)){if(y.value){const oe=(U=Y.value)===null||U===void 0?void 0:U.getPendingTmNode();oe?xe(oe):e.filterable||(ye(),et())}else if(m(),e.tag&&ke.value){const oe=g.value[0];if(oe){const ce=oe[e.valueField],{value:Ce}=c;e.multiple&&Array.isArray(Ce)&&Ce.includes(ce)||z(oe)}}}u.preventDefault();break;case"ArrowUp":if(u.preventDefault(),e.loading)return;y.value&&((ee=Y.value)===null||ee===void 0||ee.prev());break;case"ArrowDown":if(u.preventDefault(),e.loading)return;y.value?(X=Y.value)===null||X===void 0||X.next():m();break;case"Escape":y.value&&(Ln(u),ye()),(Q=T.value)===null||Q===void 0||Q.focus();break}}function et(){var u;(u=T.value)===null||u===void 0||u.focus()}function tt(){var u;(u=T.value)===null||u===void 0||u.focusInput()}function bt(){var u;y.value&&((u=M.value)===null||u===void 0||u.syncPosition())}me(),Re(ie(e,"options"),me);const yt={focus:()=>{var u;(u=T.value)===null||u===void 0||u.focus()},focusInput:()=>{var u;(u=T.value)===null||u===void 0||u.focusInput()},blur:()=>{var u;(u=T.value)===null||u===void 0||u.blur()},blurInput:()=>{var u;(u=T.value)===null||u===void 0||u.blurInput()}},ot=I(()=>{const{self:{menuBoxShadow:u}}=l.value;return{"--n-menu-box-shadow":u}}),Te=r?Pe("select",void 0,ot,e):void 0;return Object.assign(Object.assign({},yt),{mergedStatus:ne,mergedClsPrefix:t,mergedBordered:o,namespace:n,treeMate:K,isMounted:Bn(),triggerRef:T,menuRef:Y,pattern:p,uncontrolledShow:R,mergedShow:y,adjustedTo:Mt(e),uncontrolledValue:a,mergedValue:c,followerRef:M,localizedPlaceholder:J,selectedOption:L,selectedOptions:$,mergedSize:V,mergedDisabled:q,focused:h,activeWithoutMenuOpen:ke,inlineThemeDisabled:r,onTriggerInputFocus:De,onTriggerInputBlur:je,handleTriggerOrMenuResize:bt,handleMenuFocus:We,handleMenuBlur:Ue,handleMenuTabOut:Ge,handleTriggerClick:Ve,handleToggle:xe,handleDeleteOption:z,handlePatternInput:de,handleClear:pt,handleTriggerBlur:Ne,handleTriggerFocus:Fe,handleKeydown:Qe,handleMenuAfterLeave:Be,handleMenuClickOutside:$e,handleMenuScroll:gt,handleMenuKeydown:Qe,handleMenuMousedown:mt,mergedTheme:l,cssVars:r?void 0:ot,themeClass:Te==null?void 0:Te.themeClass,onRender:Te==null?void 0:Te.onRender})},render(){return d("div",{class:`${this.mergedClsPrefix}-select`},d(Io,null,{default:()=>[d(Po,null,{default:()=>d(ti,{ref:"triggerRef",inlineThemeDisabled:this.inlineThemeDisabled,status:this.mergedStatus,inputProps:this.inputProps,clsPrefix:this.mergedClsPrefix,showArrow:this.showArrow,maxTagCount:this.maxTagCount,ellipsisTagPopoverProps:this.ellipsisTagPopoverProps,bordered:this.mergedBordered,active:this.activeWithoutMenuOpen||this.mergedShow,pattern:this.pattern,placeholder:this.localizedPlaceholder,selectedOption:this.selectedOption,selectedOptions:this.selectedOptions,multiple:this.multiple,renderTag:this.renderTag,renderLabel:this.renderLabel,filterable:this.filterable,clearable:this.clearable,disabled:this.mergedDisabled,size:this.mergedSize,theme:this.mergedTheme.peers.InternalSelection,labelField:this.labelField,valueField:this.valueField,themeOverrides:this.mergedTheme.peerOverrides.InternalSelection,loading:this.loading,focused:this.focused,onClick:this.handleTriggerClick,onDeleteOption:this.handleDeleteOption,onPatternInput:this.handlePatternInput,onClear:this.handleClear,onBlur:this.handleTriggerBlur,onFocus:this.handleTriggerFocus,onKeydown:this.handleKeydown,onPatternBlur:this.onTriggerInputBlur,onPatternFocus:this.onTriggerInputFocus,onResize:this.handleTriggerOrMenuResize,ignoreComposition:this.ignoreComposition},{arrow:()=>{var e,t;return[(t=(e=this.$slots).arrow)===null||t===void 0?void 0:t.call(e)]}})}),d(ko,{ref:"followerRef",show:this.mergedShow,to:this.adjustedTo,teleportDisabled:this.adjustedTo===Mt.tdkey,containerClass:this.namespace,width:this.consistentMenuWidth?"target":void 0,minWidth:"target",placement:this.placement},{default:()=>d(Vt,{name:"fade-in-scale-up-transition",appear:this.isMounted,onAfterLeave:this.handleMenuAfterLeave},{default:()=>{var e,t,o;return this.mergedShow||this.displayDirective==="show"?((e=this.onRender)===null||e===void 0||e.call(this),_n(d(Qr,Object.assign({},this.menuProps,{ref:"menuRef",onResize:this.handleTriggerOrMenuResize,inlineThemeDisabled:this.inlineThemeDisabled,virtualScroll:this.consistentMenuWidth&&this.virtualScroll,class:[`${this.mergedClsPrefix}-select-menu`,this.themeClass,(t=this.menuProps)===null||t===void 0?void 0:t.class],clsPrefix:this.mergedClsPrefix,focusable:!0,labelField:this.labelField,valueField:this.valueField,autoPending:!0,nodeProps:this.nodeProps,theme:this.mergedTheme.peers.InternalSelectMenu,themeOverrides:this.mergedTheme.peerOverrides.InternalSelectMenu,treeMate:this.treeMate,multiple:this.multiple,size:this.menuSize,renderOption:this.renderOption,renderLabel:this.renderLabel,value:this.mergedValue,style:[(o=this.menuProps)===null||o===void 0?void 0:o.style,this.cssVars],onToggle:this.handleToggle,onScroll:this.handleMenuScroll,onFocus:this.handleMenuFocus,onBlur:this.handleMenuBlur,onKeydown:this.handleMenuKeydown,onTabOut:this.handleMenuTabOut,onMousedown:this.handleMenuMousedown,show:this.mergedShow,showCheckmark:this.showCheckmark,resetMenuOnOptionsChange:this.resetMenuOnOptionsChange,scrollbarProps:this.scrollbarProps}),{empty:()=>{var n,r;return[(r=(n=this.$slots).empty)===null||r===void 0?void 0:r.call(n)]},header:()=>{var n,r;return[(r=(n=this.$slots).header)===null||r===void 0?void 0:r.call(n)]},action:()=>{var n,r;return[(r=(n=this.$slots).action)===null||r===void 0?void 0:r.call(n)]}}),this.displayDirective==="show"?[[An,this.mergedShow],[io,this.handleMenuClickOutside,void 0,{capture:!0}]]:[[io,this.handleMenuClickOutside,void 0,{capture:!0}]])):null}})})]}))}}),Zt=Oe("n-dropdown-menu"),vt=Oe("n-dropdown"),mo=Oe("n-dropdown-option"),Eo=re({name:"DropdownDivider",props:{clsPrefix:{type:String,required:!0}},render(){return d("div",{class:`${this.clsPrefix}-dropdown-divider`})}}),si=re({name:"DropdownGroupHeader",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0}},setup(){const{showIconRef:e,hasSubmenuRef:t}=ue(Zt),{renderLabelRef:o,labelFieldRef:n,nodePropsRef:r,renderOptionRef:i}=ue(vt);return{labelField:n,showIcon:e,hasSubmenu:t,renderLabel:o,nodeProps:r,renderOption:i}},render(){var e;const{clsPrefix:t,hasSubmenu:o,showIcon:n,nodeProps:r,renderLabel:i,renderOption:l}=this,{rawNode:a}=this.tmNode,s=d("div",Object.assign({class:`${t}-dropdown-option`},r==null?void 0:r(a)),d("div",{class:`${t}-dropdown-option-body ${t}-dropdown-option-body--group`},d("div",{"data-dropdown-option":!0,class:[`${t}-dropdown-option-body__prefix`,n&&`${t}-dropdown-option-body__prefix--show-icon`]},he(a.icon)),d("div",{class:`${t}-dropdown-option-body__label`,"data-dropdown-option":!0},i?i(a):he((e=a.title)!==null&&e!==void 0?e:a[this.labelField])),d("div",{class:[`${t}-dropdown-option-body__suffix`,o&&`${t}-dropdown-option-body__suffix--has-submenu`],"data-dropdown-option":!0})));return l?l({node:s,option:a}):s}}),di=S("icon",`
 height: 1em;
 width: 1em;
 line-height: 1em;
 text-align: center;
 display: inline-block;
 position: relative;
 fill: currentColor;
`,[W("color-transition",{transition:"color .3s var(--n-bezier)"}),W("depth",{color:"var(--n-color)"},[G("svg",{opacity:"var(--n-opacity)",transition:"opacity .3s var(--n-bezier)"})]),G("svg",{height:"1em",width:"1em"})]),ci=Object.assign(Object.assign({},fe.props),{depth:[String,Number],size:[Number,String],color:String,component:[Object,Function]}),ui=re({_n_icon__:!0,name:"Icon",inheritAttrs:!1,props:ci,setup(e){const{mergedClsPrefixRef:t,inlineThemeDisabled:o}=Ie(e),n=fe("Icon","-icon",di,Kn,e,t),r=I(()=>{const{depth:l}=e,{common:{cubicBezierEaseInOut:a},self:s}=n.value;if(l!==void 0){const{color:c,[`opacity${l}Depth`]:h}=s;return{"--n-bezier":a,"--n-color":c,"--n-opacity":h}}return{"--n-bezier":a,"--n-color":"","--n-opacity":""}}),i=o?Pe("icon",I(()=>`${e.depth||"d"}`),r,e):void 0;return{mergedClsPrefix:t,mergedStyle:I(()=>{const{size:l,color:a}=e;return{fontSize:ut(l),color:a}}),cssVars:o?void 0:r,themeClass:i==null?void 0:i.themeClass,onRender:i==null?void 0:i.onRender}},render(){var e;const{$parent:t,depth:o,mergedClsPrefix:n,component:r,onRender:i,themeClass:l}=this;return!((e=t==null?void 0:t.$options)===null||e===void 0)&&e._n_icon__&&wo("icon","don't wrap `n-icon` inside `n-icon`"),i==null||i(),d("i",Ze(this.$attrs,{role:"img",class:[`${n}-icon`,l,{[`${n}-icon--depth`]:o,[`${n}-icon--color-transition`]:o!==void 0}],style:[this.cssVars,this.mergedStyle]}),r?d(r):this.$slots)}});function Et(e,t){return e.type==="submenu"||e.type===void 0&&e[t]!==void 0}function fi(e){return e.type==="group"}function Lo(e){return e.type==="divider"}function hi(e){return e.type==="render"}const Ko=re({name:"DropdownOption",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0},parentKey:{type:[String,Number],default:null},placement:{type:String,default:"right-start"},props:Object,scrollable:Boolean},setup(e){const t=ue(vt),{hoverKeyRef:o,keyboardKeyRef:n,lastToggledSubmenuKeyRef:r,pendingKeyPathRef:i,activeKeyPathRef:l,animatedRef:a,mergedShowRef:s,renderLabelRef:c,renderIconRef:h,labelFieldRef:p,childrenFieldRef:b,renderOptionRef:O,nodePropsRef:g,menuPropsRef:_}=t,B=ue(mo,null),N=ue(Zt),K=ue(Co),H=I(()=>e.tmNode.rawNode),R=I(()=>{const{value:P}=b;return Et(e.tmNode.rawNode,P)}),y=I(()=>{const{disabled:P}=e.tmNode;return P}),T=I(()=>{if(!R.value)return!1;const{key:P,disabled:V}=e.tmNode;if(V)return!1;const{value:q}=o,{value:ne}=n,{value:A}=r,{value:D}=i;return q!==null?D.includes(P):ne!==null?D.includes(P)&&D[D.length-1]!==P:A!==null?D.includes(P):!1}),M=I(()=>n.value===null&&!a.value),Y=gr(T,300,M),te=I(()=>!!(B!=null&&B.enteringSubmenuRef.value)),J=j(!1);pe(mo,{enteringSubmenuRef:J});function Z(){J.value=!0}function se(){J.value=!1}function C(){const{parentKey:P,tmNode:V}=e;V.disabled||s.value&&(r.value=P,n.value=null,o.value=V.key)}function x(){const{tmNode:P}=e;P.disabled||s.value&&o.value!==P.key&&C()}function $(P){if(e.tmNode.disabled||!s.value)return;const{relatedTarget:V}=P;V&&!Me({target:V},"dropdownOption")&&!Me({target:V},"scrollbarRail")&&(o.value=null)}function L(){const{value:P}=R,{tmNode:V}=e;s.value&&!P&&!V.disabled&&(t.doSelect(V.key,V.rawNode),t.doUpdateShow(!1))}return{labelField:p,renderLabel:c,renderIcon:h,siblingHasIcon:N.showIconRef,siblingHasSubmenu:N.hasSubmenuRef,menuProps:_,popoverBody:K,animated:a,mergedShowSubmenu:I(()=>Y.value&&!te.value),rawNode:H,hasSubmenu:R,pending:be(()=>{const{value:P}=i,{key:V}=e.tmNode;return P.includes(V)}),childActive:be(()=>{const{value:P}=l,{key:V}=e.tmNode,q=P.findIndex(ne=>V===ne);return q===-1?!1:q<P.length-1}),active:be(()=>{const{value:P}=l,{key:V}=e.tmNode,q=P.findIndex(ne=>V===ne);return q===-1?!1:q===P.length-1}),mergedDisabled:y,renderOption:O,nodeProps:g,handleClick:L,handleMouseMove:x,handleMouseEnter:C,handleMouseLeave:$,handleSubmenuBeforeEnter:Z,handleSubmenuAfterEnter:se}},render(){var e,t;const{animated:o,rawNode:n,mergedShowSubmenu:r,clsPrefix:i,siblingHasIcon:l,siblingHasSubmenu:a,renderLabel:s,renderIcon:c,renderOption:h,nodeProps:p,props:b,scrollable:O}=this;let g=null;if(r){const K=(e=this.menuProps)===null||e===void 0?void 0:e.call(this,n,n.children);g=d(Ho,Object.assign({},K,{clsPrefix:i,scrollable:this.scrollable,tmNodes:this.tmNode.children,parentKey:this.tmNode.key}))}const _={class:[`${i}-dropdown-option-body`,this.pending&&`${i}-dropdown-option-body--pending`,this.active&&`${i}-dropdown-option-body--active`,this.childActive&&`${i}-dropdown-option-body--child-active`,this.mergedDisabled&&`${i}-dropdown-option-body--disabled`],onMousemove:this.handleMouseMove,onMouseenter:this.handleMouseEnter,onMouseleave:this.handleMouseLeave,onClick:this.handleClick},B=p==null?void 0:p(n),N=d("div",Object.assign({class:[`${i}-dropdown-option`,B==null?void 0:B.class],"data-dropdown-option":!0},B),d("div",Ze(_,b),[d("div",{class:[`${i}-dropdown-option-body__prefix`,l&&`${i}-dropdown-option-body__prefix--show-icon`]},[c?c(n):he(n.icon)]),d("div",{"data-dropdown-option":!0,class:`${i}-dropdown-option-body__label`},s?s(n):he((t=n[this.labelField])!==null&&t!==void 0?t:n.title)),d("div",{"data-dropdown-option":!0,class:[`${i}-dropdown-option-body__suffix`,a&&`${i}-dropdown-option-body__suffix--has-submenu`]},this.hasSubmenu?d(ui,null,{default:()=>d(Ao,null)}):null)]),this.hasSubmenu?d(Io,null,{default:()=>[d(Po,null,{default:()=>d("div",{class:`${i}-dropdown-offset-container`},d(ko,{show:this.mergedShowSubmenu,placement:this.placement,to:O&&this.popoverBody||void 0,teleportDisabled:!O},{default:()=>d("div",{class:`${i}-dropdown-menu-wrapper`},o?d(Vt,{onBeforeEnter:this.handleSubmenuBeforeEnter,onAfterEnter:this.handleSubmenuAfterEnter,name:"fade-in-scale-up-transition",appear:!0},{default:()=>g}):g)}))})]}):null);return h?h({node:N,option:n}):N}}),vi=re({name:"NDropdownGroup",props:{clsPrefix:{type:String,required:!0},tmNode:{type:Object,required:!0},parentKey:{type:[String,Number],default:null}},render(){const{tmNode:e,parentKey:t,clsPrefix:o}=this,{children:n}=e;return d(Gt,null,d(si,{clsPrefix:o,tmNode:e,key:e.key}),n==null?void 0:n.map(r=>{const{rawNode:i}=r;return i.show===!1?null:Lo(i)?d(Eo,{clsPrefix:o,key:r.key}):r.isGroup?(wo("dropdown","`group` node is not allowed to be put in `group` node."),null):d(Ko,{clsPrefix:o,tmNode:r,parentKey:t,key:r.key})}))}}),pi=re({name:"DropdownRenderOption",props:{tmNode:{type:Object,required:!0}},render(){const{rawNode:{render:e,props:t}}=this.tmNode;return d("div",t,[e==null?void 0:e()])}}),Ho=re({name:"DropdownMenu",props:{scrollable:Boolean,showArrow:Boolean,arrowStyle:[String,Object],clsPrefix:{type:String,required:!0},tmNodes:{type:Array,default:()=>[]},parentKey:{type:[String,Number],default:null}},setup(e){const{renderIconRef:t,childrenFieldRef:o}=ue(vt);pe(Zt,{showIconRef:I(()=>{const r=t.value;return e.tmNodes.some(i=>{var l;if(i.isGroup)return(l=i.children)===null||l===void 0?void 0:l.some(({rawNode:s})=>r?r(s):s.icon);const{rawNode:a}=i;return r?r(a):a.icon})}),hasSubmenuRef:I(()=>{const{value:r}=o;return e.tmNodes.some(i=>{var l;if(i.isGroup)return(l=i.children)===null||l===void 0?void 0:l.some(({rawNode:s})=>Et(s,r));const{rawNode:a}=i;return Et(a,r)})})});const n=j(null);return pe(Dn,null),pe(jn,null),pe(Co,n),{bodyRef:n}},render(){const{parentKey:e,clsPrefix:t,scrollable:o}=this,n=this.tmNodes.map(r=>{const{rawNode:i}=r;return i.show===!1?null:hi(i)?d(pi,{tmNode:r,key:r.key}):Lo(i)?d(Eo,{clsPrefix:t,key:r.key}):fi(i)?d(vi,{clsPrefix:t,tmNode:r,parentKey:e,key:r.key}):d(Ko,{clsPrefix:t,tmNode:r,parentKey:e,key:r.key,props:i.props,scrollable:o})});return d("div",{class:[`${t}-dropdown-menu`,o&&`${t}-dropdown-menu--scrollable`],ref:"bodyRef"},o?d(Hn,{contentClass:`${t}-dropdown-menu__content`},{default:()=>n}):n,this.showArrow?sr({clsPrefix:t,arrowStyle:this.arrowStyle,arrowClass:void 0,arrowWrapperClass:void 0,arrowWrapperStyle:void 0}):null)}}),mi=S("dropdown-menu",`
 transform-origin: var(--v-transform-origin);
 background-color: var(--n-color);
 border-radius: var(--n-border-radius);
 box-shadow: var(--n-box-shadow);
 position: relative;
 transition:
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier);
`,[Wt(),S("dropdown-option",`
 position: relative;
 `,[G("a",`
 text-decoration: none;
 color: inherit;
 outline: none;
 `,[G("&::before",`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),S("dropdown-option-body",`
 display: flex;
 cursor: pointer;
 position: relative;
 height: var(--n-option-height);
 line-height: var(--n-option-height);
 font-size: var(--n-font-size);
 color: var(--n-option-text-color);
 transition: color .3s var(--n-bezier);
 `,[G("&::before",`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 left: 4px;
 right: 4px;
 transition: background-color .3s var(--n-bezier);
 border-radius: var(--n-border-radius);
 `),Se("disabled",[W("pending",`
 color: var(--n-option-text-color-hover);
 `,[k("prefix, suffix",`
 color: var(--n-option-text-color-hover);
 `),G("&::before","background-color: var(--n-option-color-hover);")]),W("active",`
 color: var(--n-option-text-color-active);
 `,[k("prefix, suffix",`
 color: var(--n-option-text-color-active);
 `),G("&::before","background-color: var(--n-option-color-active);")]),W("child-active",`
 color: var(--n-option-text-color-child-active);
 `,[k("prefix, suffix",`
 color: var(--n-option-text-color-child-active);
 `)])]),W("disabled",`
 cursor: not-allowed;
 opacity: var(--n-option-opacity-disabled);
 `),W("group",`
 font-size: calc(var(--n-font-size) - 1px);
 color: var(--n-group-header-text-color);
 `,[k("prefix",`
 width: calc(var(--n-option-prefix-width) / 2);
 `,[W("show-icon",`
 width: calc(var(--n-option-icon-prefix-width) / 2);
 `)])]),k("prefix",`
 width: var(--n-option-prefix-width);
 display: flex;
 justify-content: center;
 align-items: center;
 color: var(--n-prefix-color);
 transition: color .3s var(--n-bezier);
 z-index: 1;
 `,[W("show-icon",`
 width: var(--n-option-icon-prefix-width);
 `),S("icon",`
 font-size: var(--n-option-icon-size);
 `)]),k("label",`
 white-space: nowrap;
 flex: 1;
 z-index: 1;
 `),k("suffix",`
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
 `,[W("has-submenu",`
 width: var(--n-option-icon-suffix-width);
 `),S("icon",`
 font-size: var(--n-option-icon-size);
 `)]),S("dropdown-menu","pointer-events: all;")]),S("dropdown-offset-container",`
 pointer-events: none;
 position: absolute;
 left: 0;
 right: 0;
 top: -4px;
 bottom: -4px;
 `)]),S("dropdown-divider",`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 4px 0;
 `),S("dropdown-menu-wrapper",`
 transform-origin: var(--v-transform-origin);
 width: fit-content;
 `),G(">",[S("scrollbar",`
 height: inherit;
 max-height: inherit;
 `)]),Se("scrollable",`
 padding: var(--n-padding);
 `),W("scrollable",[k("content",`
 padding: var(--n-padding);
 `)])]),gi={animated:{type:Boolean,default:!0},keyboard:{type:Boolean,default:!0},size:String,inverted:Boolean,placement:{type:String,default:"bottom"},onSelect:[Function,Array],options:{type:Array,default:()=>[]},menuProps:Function,showArrow:Boolean,renderLabel:Function,renderIcon:Function,renderOption:Function,nodeProps:Function,labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},value:[String,Number]},bi=Object.keys(Oo),yi=Object.assign(Object.assign(Object.assign({},Oo),gi),fe.props),xi=re({name:"Dropdown",inheritAttrs:!1,props:yi,setup(e){const t=j(!1),o=Ke(ie(e,"show"),t),n=I(()=>{const{keyField:x,childrenField:$}=e;return Xe(e.options,{getKey(L){return L[x]},getDisabled(L){return L.disabled===!0},getIgnored(L){return L.type==="divider"||L.type==="render"},getChildren(L){return L[$]}})}),r=I(()=>n.value.treeNodes),i=j(null),l=j(null),a=j(null),s=I(()=>{var x,$,L;return(L=($=(x=i.value)!==null&&x!==void 0?x:l.value)!==null&&$!==void 0?$:a.value)!==null&&L!==void 0?L:null}),c=I(()=>n.value.getPath(s.value).keyPath),h=I(()=>n.value.getPath(e.value).keyPath),p=be(()=>e.keyboard&&o.value);mr({keydown:{ArrowUp:{prevent:!0,handler:M},ArrowRight:{prevent:!0,handler:T},ArrowDown:{prevent:!0,handler:Y},ArrowLeft:{prevent:!0,handler:y},Enter:{prevent:!0,handler:te},Escape:R}},p);const{mergedClsPrefixRef:b,inlineThemeDisabled:O,mergedComponentPropsRef:g}=Ie(e),_=I(()=>{var x,$;return e.size||(($=(x=g==null?void 0:g.value)===null||x===void 0?void 0:x.Dropdown)===null||$===void 0?void 0:$.size)||"medium"}),B=fe("Dropdown","-dropdown",mi,Vn,e,b);pe(vt,{labelFieldRef:ie(e,"labelField"),childrenFieldRef:ie(e,"childrenField"),renderLabelRef:ie(e,"renderLabel"),renderIconRef:ie(e,"renderIcon"),hoverKeyRef:i,keyboardKeyRef:l,lastToggledSubmenuKeyRef:a,pendingKeyPathRef:c,activeKeyPathRef:h,animatedRef:ie(e,"animated"),mergedShowRef:o,nodePropsRef:ie(e,"nodeProps"),renderOptionRef:ie(e,"renderOption"),menuPropsRef:ie(e,"menuProps"),doSelect:N,doUpdateShow:K}),Re(o,x=>{!e.animated&&!x&&H()});function N(x,$){const{onSelect:L}=e;L&&ae(L,x,$)}function K(x){const{"onUpdate:show":$,onUpdateShow:L}=e;$&&ae($,x),L&&ae(L,x),t.value=x}function H(){i.value=null,l.value=null,a.value=null}function R(){K(!1)}function y(){Z("left")}function T(){Z("right")}function M(){Z("up")}function Y(){Z("down")}function te(){const x=J();x!=null&&x.isLeaf&&o.value&&(N(x.key,x.rawNode),K(!1))}function J(){var x;const{value:$}=n,{value:L}=s;return!$||L===null?null:(x=$.getNode(L))!==null&&x!==void 0?x:null}function Z(x){const{value:$}=s,{value:{getFirstAvailableNode:L}}=n;let P=null;if($===null){const V=L();V!==null&&(P=V.key)}else{const V=J();if(V){let q;switch(x){case"down":q=V.getNext();break;case"up":q=V.getPrev();break;case"right":q=V.getChild();break;case"left":q=V.getParent();break}q&&(P=q.key)}}P!==null&&(i.value=null,l.value=P)}const se=I(()=>{const{inverted:x}=e,$=_.value,{common:{cubicBezierEaseInOut:L},self:P}=B.value,{padding:V,dividerColor:q,borderRadius:ne,optionOpacityDisabled:A,[ge("optionIconSuffixWidth",$)]:D,[ge("optionSuffixWidth",$)]:v,[ge("optionIconPrefixWidth",$)]:f,[ge("optionPrefixWidth",$)]:w,[ge("fontSize",$)]:le,[ge("optionHeight",$)]:me,[ge("optionIconSize",$)]:we}=P,m={"--n-bezier":L,"--n-font-size":le,"--n-padding":V,"--n-border-radius":ne,"--n-option-height":me,"--n-option-prefix-width":w,"--n-option-icon-prefix-width":f,"--n-option-suffix-width":v,"--n-option-icon-suffix-width":D,"--n-option-icon-size":we,"--n-divider-color":q,"--n-option-opacity-disabled":A};return x?(m["--n-color"]=P.colorInverted,m["--n-option-color-hover"]=P.optionColorHoverInverted,m["--n-option-color-active"]=P.optionColorActiveInverted,m["--n-option-text-color"]=P.optionTextColorInverted,m["--n-option-text-color-hover"]=P.optionTextColorHoverInverted,m["--n-option-text-color-active"]=P.optionTextColorActiveInverted,m["--n-option-text-color-child-active"]=P.optionTextColorChildActiveInverted,m["--n-prefix-color"]=P.prefixColorInverted,m["--n-suffix-color"]=P.suffixColorInverted,m["--n-group-header-text-color"]=P.groupHeaderTextColorInverted):(m["--n-color"]=P.color,m["--n-option-color-hover"]=P.optionColorHover,m["--n-option-color-active"]=P.optionColorActive,m["--n-option-text-color"]=P.optionTextColor,m["--n-option-text-color-hover"]=P.optionTextColorHover,m["--n-option-text-color-active"]=P.optionTextColorActive,m["--n-option-text-color-child-active"]=P.optionTextColorChildActive,m["--n-prefix-color"]=P.prefixColor,m["--n-suffix-color"]=P.suffixColor,m["--n-group-header-text-color"]=P.groupHeaderTextColor),m}),C=O?Pe("dropdown",I(()=>`${_.value[0]}${e.inverted?"i":""}`),se,e):void 0;return{mergedClsPrefix:b,mergedTheme:B,mergedSize:_,tmNodes:r,mergedShow:o,handleAfterLeave:()=>{e.animated&&H()},doUpdateShow:K,cssVars:O?void 0:se,themeClass:C==null?void 0:C.themeClass,onRender:C==null?void 0:C.onRender}},render(){const e=(n,r,i,l,a)=>{var s;const{mergedClsPrefix:c,menuProps:h}=this;(s=this.onRender)===null||s===void 0||s.call(this);const p=(h==null?void 0:h(void 0,this.tmNodes.map(O=>O.rawNode)))||{},b={ref:Sr(r),class:[n,`${c}-dropdown`,`${c}-dropdown--${this.mergedSize}-size`,this.themeClass],clsPrefix:c,tmNodes:this.tmNodes,style:[...i,this.cssVars],showArrow:this.showArrow,arrowStyle:this.arrowStyle,scrollable:this.scrollable,onMouseenter:l,onMouseleave:a};return d(Ho,Ze(this.$attrs,b,p))},{mergedTheme:t}=this,o={show:this.mergedShow,theme:t.peers.Popover,themeOverrides:t.peerOverrides.Popover,internalOnAfterLeave:this.handleAfterLeave,internalRenderBody:e,onUpdateShow:this.doUpdateShow,"onUpdate:show":void 0};return d(zo,Object.assign({},dt(this.$props,bi),o),{trigger:()=>{var n,r;return(r=(n=this.$slots).default)===null||r===void 0?void 0:r.call(n)}})}});function wi(e){const{baseColor:t,textColor2:o,bodyColor:n,cardColor:r,dividerColor:i,actionColor:l,scrollbarColor:a,scrollbarColorHover:s,invertedColor:c}=e;return{textColor:o,textColorInverted:"#FFF",color:n,colorEmbedded:l,headerColor:r,headerColorInverted:c,footerColor:l,footerColorInverted:c,headerBorderColor:i,headerBorderColorInverted:c,footerBorderColor:i,footerBorderColorInverted:c,siderBorderColor:i,siderBorderColorInverted:c,siderColor:r,siderColorInverted:c,siderToggleButtonBorder:`1px solid ${i}`,siderToggleButtonColor:t,siderToggleButtonIconColor:o,siderToggleButtonIconColorInverted:o,siderToggleBarColor:lo(n,a),siderToggleBarColorHover:lo(n,s),__invertScrollbar:"true"}}const Jt=Wn({name:"Layout",common:Gn,peers:{Scrollbar:Un},self:wi}),Do=Oe("n-layout-sider"),Qt={type:String,default:"static"},Ci=S("layout",`
 color: var(--n-text-color);
 background-color: var(--n-color);
 box-sizing: border-box;
 position: relative;
 z-index: auto;
 flex: auto;
 overflow: hidden;
 transition:
 box-shadow .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier);
`,[S("layout-scroll-container",`
 overflow-x: hidden;
 box-sizing: border-box;
 height: 100%;
 `),W("absolute-positioned",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),Si={embedded:Boolean,position:Qt,nativeScrollbar:{type:Boolean,default:!0},scrollbarProps:Object,onScroll:Function,contentClass:String,contentStyle:{type:[String,Object],default:""},hasSider:Boolean,siderPlacement:{type:String,default:"left"}},jo=Oe("n-layout");function Ri(e){return re({name:"Layout",props:Object.assign(Object.assign({},fe.props),Si),setup(t){const o=j(null),n=j(null),{mergedClsPrefixRef:r,inlineThemeDisabled:i}=Ie(t),l=fe("Layout","-layout",Ci,Jt,t,r);function a(_,B){if(t.nativeScrollbar){const{value:N}=o;N&&(B===void 0?N.scrollTo(_):N.scrollTo(_,B))}else{const{value:N}=n;N&&N.scrollTo(_,B)}}pe(jo,t);let s=0,c=0;const h=_=>{var B;const N=_.target;s=N.scrollLeft,c=N.scrollTop,(B=t.onScroll)===null||B===void 0||B.call(t,_)};So(()=>{if(t.nativeScrollbar){const _=o.value;_&&(_.scrollTop=c,_.scrollLeft=s)}});const p={display:"flex",flexWrap:"nowrap",width:"100%",flexDirection:"row"},b={scrollTo:a},O=I(()=>{const{common:{cubicBezierEaseInOut:_},self:B}=l.value;return{"--n-bezier":_,"--n-color":t.embedded?B.colorEmbedded:B.color,"--n-text-color":B.textColor}}),g=i?Pe("layout",I(()=>t.embedded?"e":""),O,t):void 0;return Object.assign({mergedClsPrefix:r,scrollableElRef:o,scrollbarInstRef:n,hasSiderStyle:p,mergedTheme:l,handleNativeElScroll:h,cssVars:i?void 0:O,themeClass:g==null?void 0:g.themeClass,onRender:g==null?void 0:g.onRender},b)},render(){var t;const{mergedClsPrefix:o,hasSider:n}=this;(t=this.onRender)===null||t===void 0||t.call(this);const r=n?this.hasSiderStyle:void 0,i=[this.themeClass,e,`${o}-layout`,`${o}-layout--${this.position}-positioned`];return d("div",{class:i,style:this.cssVars},this.nativeScrollbar?d("div",{ref:"scrollableElRef",class:[`${o}-layout-scroll-container`,this.contentClass],style:[this.contentStyle,r],onScroll:this.handleNativeElScroll},this.$slots):d(Ut,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,contentClass:this.contentClass,contentStyle:[this.contentStyle,r]}),this.$slots))}})}const Nt=Ri(!1),zi=S("layout-header",`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 box-sizing: border-box;
 width: 100%;
 background-color: var(--n-color);
 color: var(--n-text-color);
`,[W("absolute-positioned",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 `),W("bordered",`
 border-bottom: solid 1px var(--n-border-color);
 `)]),Ii={position:Qt,inverted:Boolean,bordered:{type:Boolean,default:!1}},Pi=re({name:"LayoutHeader",props:Object.assign(Object.assign({},fe.props),Ii),setup(e){const{mergedClsPrefixRef:t,inlineThemeDisabled:o}=Ie(e),n=fe("Layout","-layout-header",zi,Jt,e,t),r=I(()=>{const{common:{cubicBezierEaseInOut:l},self:a}=n.value,s={"--n-bezier":l};return e.inverted?(s["--n-color"]=a.headerColorInverted,s["--n-text-color"]=a.textColorInverted,s["--n-border-color"]=a.headerBorderColorInverted):(s["--n-color"]=a.headerColor,s["--n-text-color"]=a.textColor,s["--n-border-color"]=a.headerBorderColor),s}),i=o?Pe("layout-header",I(()=>e.inverted?"a":"b"),r,e):void 0;return{mergedClsPrefix:t,cssVars:o?void 0:r,themeClass:i==null?void 0:i.themeClass,onRender:i==null?void 0:i.onRender}},render(){var e;const{mergedClsPrefix:t}=this;return(e=this.onRender)===null||e===void 0||e.call(this),d("div",{class:[`${t}-layout-header`,this.themeClass,this.position&&`${t}-layout-header--${this.position}-positioned`,this.bordered&&`${t}-layout-header--bordered`],style:this.cssVars},this.$slots)}}),ki=S("layout-sider",`
 flex-shrink: 0;
 box-sizing: border-box;
 position: relative;
 z-index: 1;
 color: var(--n-text-color);
 transition:
 color .3s var(--n-bezier),
 border-color .3s var(--n-bezier),
 min-width .3s var(--n-bezier),
 max-width .3s var(--n-bezier),
 transform .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 background-color: var(--n-color);
 display: flex;
 justify-content: flex-end;
`,[W("bordered",[k("border",`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 width: 1px;
 background-color: var(--n-border-color);
 transition: background-color .3s var(--n-bezier);
 `)]),k("left-placement",[W("bordered",[k("border",`
 right: 0;
 `)])]),W("right-placement",`
 justify-content: flex-start;
 `,[W("bordered",[k("border",`
 left: 0;
 `)]),W("collapsed",[S("layout-toggle-button",[S("base-icon",`
 transform: rotate(180deg);
 `)]),S("layout-toggle-bar",[G("&:hover",[k("top",{transform:"rotate(-12deg) scale(1.15) translateY(-2px)"}),k("bottom",{transform:"rotate(12deg) scale(1.15) translateY(2px)"})])])]),S("layout-toggle-button",`
 left: 0;
 transform: translateX(-50%) translateY(-50%);
 `,[S("base-icon",`
 transform: rotate(0);
 `)]),S("layout-toggle-bar",`
 left: -28px;
 transform: rotate(180deg);
 `,[G("&:hover",[k("top",{transform:"rotate(12deg) scale(1.15) translateY(-2px)"}),k("bottom",{transform:"rotate(-12deg) scale(1.15) translateY(2px)"})])])]),W("collapsed",[S("layout-toggle-bar",[G("&:hover",[k("top",{transform:"rotate(-12deg) scale(1.15) translateY(-2px)"}),k("bottom",{transform:"rotate(12deg) scale(1.15) translateY(2px)"})])]),S("layout-toggle-button",[S("base-icon",`
 transform: rotate(0);
 `)])]),S("layout-toggle-button",`
 transition:
 color .3s var(--n-bezier),
 right .3s var(--n-bezier),
 left .3s var(--n-bezier),
 border-color .3s var(--n-bezier),
 background-color .3s var(--n-bezier);
 cursor: pointer;
 width: 24px;
 height: 24px;
 position: absolute;
 top: 50%;
 right: 0;
 border-radius: 50%;
 display: flex;
 align-items: center;
 justify-content: center;
 font-size: 18px;
 color: var(--n-toggle-button-icon-color);
 border: var(--n-toggle-button-border);
 background-color: var(--n-toggle-button-color);
 box-shadow: 0 2px 4px 0px rgba(0, 0, 0, .06);
 transform: translateX(50%) translateY(-50%);
 z-index: 1;
 `,[S("base-icon",`
 transition: transform .3s var(--n-bezier);
 transform: rotate(180deg);
 `)]),S("layout-toggle-bar",`
 cursor: pointer;
 height: 72px;
 width: 32px;
 position: absolute;
 top: calc(50% - 36px);
 right: -28px;
 `,[k("top, bottom",`
 position: absolute;
 width: 4px;
 border-radius: 2px;
 height: 38px;
 left: 14px;
 transition: 
 background-color .3s var(--n-bezier),
 transform .3s var(--n-bezier);
 `),k("bottom",`
 position: absolute;
 top: 34px;
 `),G("&:hover",[k("top",{transform:"rotate(12deg) scale(1.15) translateY(-2px)"}),k("bottom",{transform:"rotate(-12deg) scale(1.15) translateY(2px)"})]),k("top, bottom",{backgroundColor:"var(--n-toggle-bar-color)"}),G("&:hover",[k("top, bottom",{backgroundColor:"var(--n-toggle-bar-color-hover)"})])]),k("border",`
 position: absolute;
 top: 0;
 right: 0;
 bottom: 0;
 width: 1px;
 transition: background-color .3s var(--n-bezier);
 `),S("layout-sider-scroll-container",`
 flex-grow: 1;
 flex-shrink: 0;
 box-sizing: border-box;
 height: 100%;
 opacity: 0;
 transition: opacity .3s var(--n-bezier);
 max-width: 100%;
 `),W("show-content",[S("layout-sider-scroll-container",{opacity:1})]),W("absolute-positioned",`
 position: absolute;
 left: 0;
 top: 0;
 bottom: 0;
 `)]),Ti=re({props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return d("div",{onClick:this.onClick,class:`${e}-layout-toggle-bar`},d("div",{class:`${e}-layout-toggle-bar__top`}),d("div",{class:`${e}-layout-toggle-bar__bottom`}))}}),Oi=re({name:"LayoutToggleButton",props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return d("div",{class:`${e}-layout-toggle-button`,onClick:this.onClick},d(jt,{clsPrefix:e},{default:()=>d(Ao,null)}))}}),Ni={position:Qt,bordered:Boolean,collapsedWidth:{type:Number,default:48},width:{type:[Number,String],default:272},contentClass:String,contentStyle:{type:[String,Object],default:""},collapseMode:{type:String,default:"transform"},collapsed:{type:Boolean,default:void 0},defaultCollapsed:Boolean,showCollapsedContent:{type:Boolean,default:!0},showTrigger:{type:[Boolean,String],default:!1},nativeScrollbar:{type:Boolean,default:!0},inverted:Boolean,scrollbarProps:Object,triggerClass:String,triggerStyle:[String,Object],collapsedTriggerClass:String,collapsedTriggerStyle:[String,Object],"onUpdate:collapsed":[Function,Array],onUpdateCollapsed:[Function,Array],onAfterEnter:Function,onAfterLeave:Function,onExpand:[Function,Array],onCollapse:[Function,Array],onScroll:Function},Fi=re({name:"LayoutSider",props:Object.assign(Object.assign({},fe.props),Ni),setup(e){const t=ue(jo),o=j(null),n=j(null),r=j(e.defaultCollapsed),i=Ke(ie(e,"collapsed"),r),l=I(()=>ut(i.value?e.collapsedWidth:e.width)),a=I(()=>e.collapseMode!=="transform"?{}:{minWidth:ut(e.width)}),s=I(()=>t?t.siderPlacement:"left");function c(y,T){if(e.nativeScrollbar){const{value:M}=o;M&&(T===void 0?M.scrollTo(y):M.scrollTo(y,T))}else{const{value:M}=n;M&&M.scrollTo(y,T)}}function h(){const{"onUpdate:collapsed":y,onUpdateCollapsed:T,onExpand:M,onCollapse:Y}=e,{value:te}=i;T&&ae(T,!te),y&&ae(y,!te),r.value=!te,te?M&&ae(M):Y&&ae(Y)}let p=0,b=0;const O=y=>{var T;const M=y.target;p=M.scrollLeft,b=M.scrollTop,(T=e.onScroll)===null||T===void 0||T.call(e,y)};So(()=>{if(e.nativeScrollbar){const y=o.value;y&&(y.scrollTop=b,y.scrollLeft=p)}}),pe(Do,{collapsedRef:i,collapseModeRef:ie(e,"collapseMode")});const{mergedClsPrefixRef:g,inlineThemeDisabled:_}=Ie(e),B=fe("Layout","-layout-sider",ki,Jt,e,g);function N(y){var T,M;y.propertyName==="max-width"&&(i.value?(T=e.onAfterLeave)===null||T===void 0||T.call(e):(M=e.onAfterEnter)===null||M===void 0||M.call(e))}const K={scrollTo:c},H=I(()=>{const{common:{cubicBezierEaseInOut:y},self:T}=B.value,{siderToggleButtonColor:M,siderToggleButtonBorder:Y,siderToggleBarColor:te,siderToggleBarColorHover:J}=T,Z={"--n-bezier":y,"--n-toggle-button-color":M,"--n-toggle-button-border":Y,"--n-toggle-bar-color":te,"--n-toggle-bar-color-hover":J};return e.inverted?(Z["--n-color"]=T.siderColorInverted,Z["--n-text-color"]=T.textColorInverted,Z["--n-border-color"]=T.siderBorderColorInverted,Z["--n-toggle-button-icon-color"]=T.siderToggleButtonIconColorInverted,Z.__invertScrollbar=T.__invertScrollbar):(Z["--n-color"]=T.siderColor,Z["--n-text-color"]=T.textColor,Z["--n-border-color"]=T.siderBorderColor,Z["--n-toggle-button-icon-color"]=T.siderToggleButtonIconColor),Z}),R=_?Pe("layout-sider",I(()=>e.inverted?"a":"b"),H,e):void 0;return Object.assign({scrollableElRef:o,scrollbarInstRef:n,mergedClsPrefix:g,mergedTheme:B,styleMaxWidth:l,mergedCollapsed:i,scrollContainerStyle:a,siderPlacement:s,handleNativeElScroll:O,handleTransitionend:N,handleTriggerClick:h,inlineThemeDisabled:_,cssVars:H,themeClass:R==null?void 0:R.themeClass,onRender:R==null?void 0:R.onRender},K)},render(){var e;const{mergedClsPrefix:t,mergedCollapsed:o,showTrigger:n}=this;return(e=this.onRender)===null||e===void 0||e.call(this),d("aside",{class:[`${t}-layout-sider`,this.themeClass,`${t}-layout-sider--${this.position}-positioned`,`${t}-layout-sider--${this.siderPlacement}-placement`,this.bordered&&`${t}-layout-sider--bordered`,o&&`${t}-layout-sider--collapsed`,(!o||this.showCollapsedContent)&&`${t}-layout-sider--show-content`],onTransitionend:this.handleTransitionend,style:[this.inlineThemeDisabled?void 0:this.cssVars,{maxWidth:this.styleMaxWidth,width:ut(this.width)}]},this.nativeScrollbar?d("div",{class:[`${t}-layout-sider-scroll-container`,this.contentClass],onScroll:this.handleNativeElScroll,style:[this.scrollContainerStyle,{overflow:"auto"},this.contentStyle],ref:"scrollableElRef"},this.$slots):d(Ut,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",style:this.scrollContainerStyle,contentStyle:this.contentStyle,contentClass:this.contentClass,theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,builtinThemeOverrides:this.inverted&&this.cssVars.__invertScrollbar==="true"?{colorHover:"rgba(255, 255, 255, .4)",color:"rgba(255, 255, 255, .3)"}:void 0}),this.$slots),n?n==="bar"?d(Ti,{clsPrefix:t,class:o?this.collapsedTriggerClass:this.triggerClass,style:o?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):d(Oi,{clsPrefix:t,class:o?this.collapsedTriggerClass:this.triggerClass,style:o?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):null,this.bordered?d("div",{class:`${t}-layout-sider__border`}):null)}}),Je=Oe("n-menu"),Vo=Oe("n-submenu"),eo=Oe("n-menu-item-group"),go=[G("&::before","background-color: var(--n-item-color-hover);"),k("arrow",`
 color: var(--n-arrow-color-hover);
 `),k("icon",`
 color: var(--n-item-icon-color-hover);
 `),S("menu-item-content-header",`
 color: var(--n-item-text-color-hover);
 `,[G("a",`
 color: var(--n-item-text-color-hover);
 `),k("extra",`
 color: var(--n-item-text-color-hover);
 `)])],bo=[k("icon",`
 color: var(--n-item-icon-color-hover-horizontal);
 `),S("menu-item-content-header",`
 color: var(--n-item-text-color-hover-horizontal);
 `,[G("a",`
 color: var(--n-item-text-color-hover-horizontal);
 `),k("extra",`
 color: var(--n-item-text-color-hover-horizontal);
 `)])],_i=G([S("menu",`
 background-color: var(--n-color);
 color: var(--n-item-text-color);
 overflow: hidden;
 transition: background-color .3s var(--n-bezier);
 box-sizing: border-box;
 font-size: var(--n-font-size);
 padding-bottom: 6px;
 `,[W("horizontal",`
 max-width: 100%;
 width: 100%;
 display: flex;
 overflow: hidden;
 padding-bottom: 0;
 `,[S("submenu","margin: 0;"),S("menu-item","margin: 0;"),S("menu-item-content",`
 padding: 0 20px;
 border-bottom: 2px solid #0000;
 `,[G("&::before","display: none;"),W("selected","border-bottom: 2px solid var(--n-border-color-horizontal)")]),S("menu-item-content",[W("selected",[k("icon","color: var(--n-item-icon-color-active-horizontal);"),S("menu-item-content-header",`
 color: var(--n-item-text-color-active-horizontal);
 `,[G("a","color: var(--n-item-text-color-active-horizontal);"),k("extra","color: var(--n-item-text-color-active-horizontal);")])]),W("child-active",`
 border-bottom: 2px solid var(--n-border-color-horizontal);
 `,[S("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-horizontal);
 `,[G("a",`
 color: var(--n-item-text-color-child-active-horizontal);
 `),k("extra",`
 color: var(--n-item-text-color-child-active-horizontal);
 `)]),k("icon",`
 color: var(--n-item-icon-color-child-active-horizontal);
 `)]),Se("disabled",[Se("selected, child-active",[G("&:focus-within",bo)]),W("selected",[Ae(null,[k("icon","color: var(--n-item-icon-color-active-hover-horizontal);"),S("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover-horizontal);
 `,[G("a","color: var(--n-item-text-color-active-hover-horizontal);"),k("extra","color: var(--n-item-text-color-active-hover-horizontal);")])])]),W("child-active",[Ae(null,[k("icon","color: var(--n-item-icon-color-child-active-hover-horizontal);"),S("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover-horizontal);
 `,[G("a","color: var(--n-item-text-color-child-active-hover-horizontal);"),k("extra","color: var(--n-item-text-color-child-active-hover-horizontal);")])])]),Ae("border-bottom: 2px solid var(--n-border-color-horizontal);",bo)]),S("menu-item-content-header",[G("a","color: var(--n-item-text-color-horizontal);")])])]),Se("responsive",[S("menu-item-content-header",`
 overflow: hidden;
 text-overflow: ellipsis;
 `)]),W("collapsed",[S("menu-item-content",[W("selected",[G("&::before",`
 background-color: var(--n-item-color-active-collapsed) !important;
 `)]),S("menu-item-content-header","opacity: 0;"),k("arrow","opacity: 0;"),k("icon","color: var(--n-item-icon-color-collapsed);")])]),S("menu-item",`
 height: var(--n-item-height);
 margin-top: 6px;
 position: relative;
 `),S("menu-item-content",`
 box-sizing: border-box;
 line-height: 1.75;
 height: 100%;
 display: grid;
 grid-template-areas: "icon content arrow";
 grid-template-columns: auto 1fr auto;
 align-items: center;
 cursor: pointer;
 position: relative;
 padding-right: 18px;
 transition:
 background-color .3s var(--n-bezier),
 padding-left .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 `,[G("> *","z-index: 1;"),G("&::before",`
 z-index: auto;
 content: "";
 background-color: #0000;
 position: absolute;
 left: 8px;
 right: 8px;
 top: 0;
 bottom: 0;
 pointer-events: none;
 border-radius: var(--n-border-radius);
 transition: background-color .3s var(--n-bezier);
 `),W("disabled",`
 opacity: .45;
 cursor: not-allowed;
 `),W("collapsed",[k("arrow","transform: rotate(0);")]),W("selected",[G("&::before","background-color: var(--n-item-color-active);"),k("arrow","color: var(--n-arrow-color-active);"),k("icon","color: var(--n-item-icon-color-active);"),S("menu-item-content-header",`
 color: var(--n-item-text-color-active);
 `,[G("a","color: var(--n-item-text-color-active);"),k("extra","color: var(--n-item-text-color-active);")])]),W("child-active",[S("menu-item-content-header",`
 color: var(--n-item-text-color-child-active);
 `,[G("a",`
 color: var(--n-item-text-color-child-active);
 `),k("extra",`
 color: var(--n-item-text-color-child-active);
 `)]),k("arrow",`
 color: var(--n-arrow-color-child-active);
 `),k("icon",`
 color: var(--n-item-icon-color-child-active);
 `)]),Se("disabled",[Se("selected, child-active",[G("&:focus-within",go)]),W("selected",[Ae(null,[k("arrow","color: var(--n-arrow-color-active-hover);"),k("icon","color: var(--n-item-icon-color-active-hover);"),S("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover);
 `,[G("a","color: var(--n-item-text-color-active-hover);"),k("extra","color: var(--n-item-text-color-active-hover);")])])]),W("child-active",[Ae(null,[k("arrow","color: var(--n-arrow-color-child-active-hover);"),k("icon","color: var(--n-item-icon-color-child-active-hover);"),S("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover);
 `,[G("a","color: var(--n-item-text-color-child-active-hover);"),k("extra","color: var(--n-item-text-color-child-active-hover);")])])]),W("selected",[Ae(null,[G("&::before","background-color: var(--n-item-color-active-hover);")])]),Ae(null,go)]),k("icon",`
 grid-area: icon;
 color: var(--n-item-icon-color);
 transition:
 color .3s var(--n-bezier),
 font-size .3s var(--n-bezier),
 margin-right .3s var(--n-bezier);
 box-sizing: content-box;
 display: inline-flex;
 align-items: center;
 justify-content: center;
 `),k("arrow",`
 grid-area: arrow;
 font-size: 16px;
 color: var(--n-arrow-color);
 transform: rotate(180deg);
 opacity: 1;
 transition:
 color .3s var(--n-bezier),
 transform 0.2s var(--n-bezier),
 opacity 0.2s var(--n-bezier);
 `),S("menu-item-content-header",`
 grid-area: content;
 transition:
 color .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 opacity: 1;
 white-space: nowrap;
 color: var(--n-item-text-color);
 `,[G("a",`
 outline: none;
 text-decoration: none;
 transition: color .3s var(--n-bezier);
 color: var(--n-item-text-color);
 `,[G("&::before",`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),k("extra",`
 font-size: .93em;
 color: var(--n-group-text-color);
 transition: color .3s var(--n-bezier);
 `)])]),S("submenu",`
 cursor: pointer;
 position: relative;
 margin-top: 6px;
 `,[S("menu-item-content",`
 height: var(--n-item-height);
 `),S("submenu-children",`
 overflow: hidden;
 padding: 0;
 `,[qn({duration:".2s"})])]),S("menu-item-group",[S("menu-item-group-title",`
 margin-top: 6px;
 color: var(--n-group-text-color);
 cursor: default;
 font-size: .93em;
 height: 36px;
 display: flex;
 align-items: center;
 transition:
 padding-left .3s var(--n-bezier),
 color .3s var(--n-bezier);
 `)])]),S("menu-tooltip",[G("a",`
 color: inherit;
 text-decoration: none;
 `)]),S("menu-divider",`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 6px 18px;
 `)]);function Ae(e,t){return[W("hover",e,t),G("&:hover",e,t)]}const Wo=re({name:"MenuOptionContent",props:{collapsed:Boolean,disabled:Boolean,title:[String,Function],icon:Function,extra:[String,Function],showArrow:Boolean,childActive:Boolean,hover:Boolean,paddingLeft:Number,selected:Boolean,maxIconSize:{type:Number,required:!0},activeIconSize:{type:Number,required:!0},iconMarginRight:{type:Number,required:!0},clsPrefix:{type:String,required:!0},onClick:Function,tmNode:{type:Object,required:!0},isEllipsisPlaceholder:Boolean},setup(e){const{props:t}=ue(Je);return{menuProps:t,style:I(()=>{const{paddingLeft:o}=e;return{paddingLeft:o&&`${o}px`}}),iconStyle:I(()=>{const{maxIconSize:o,activeIconSize:n,iconMarginRight:r}=e;return{width:`${o}px`,height:`${o}px`,fontSize:`${n}px`,marginRight:`${r}px`}})}},render(){const{clsPrefix:e,tmNode:t,menuProps:{renderIcon:o,renderLabel:n,renderExtra:r,expandIcon:i}}=this,l=o?o(t.rawNode):he(this.icon);return d("div",{onClick:a=>{var s;(s=this.onClick)===null||s===void 0||s.call(this,a)},role:"none",class:[`${e}-menu-item-content`,{[`${e}-menu-item-content--selected`]:this.selected,[`${e}-menu-item-content--collapsed`]:this.collapsed,[`${e}-menu-item-content--child-active`]:this.childActive,[`${e}-menu-item-content--disabled`]:this.disabled,[`${e}-menu-item-content--hover`]:this.hover}],style:this.style},l&&d("div",{class:`${e}-menu-item-content__icon`,style:this.iconStyle,role:"none"},[l]),d("div",{class:`${e}-menu-item-content-header`,role:"none"},this.isEllipsisPlaceholder?this.title:n?n(t.rawNode):he(this.title),this.extra||r?d("span",{class:`${e}-menu-item-content-header__extra`}," ",r?r(t.rawNode):he(this.extra)):null),this.showArrow?d(jt,{ariaHidden:!0,class:`${e}-menu-item-content__arrow`,clsPrefix:e},{default:()=>i?i(t.rawNode):d(zr,null)}):null)}}),st=8;function to(e){const t=ue(Je),{props:o,mergedCollapsedRef:n}=t,r=ue(Vo,null),i=ue(eo,null),l=I(()=>o.mode==="horizontal"),a=I(()=>l.value?o.dropdownPlacement:"tmNodes"in e?"right-start":"right"),s=I(()=>{var b;return Math.max((b=o.collapsedIconSize)!==null&&b!==void 0?b:o.iconSize,o.iconSize)}),c=I(()=>{var b;return!l.value&&e.root&&n.value&&(b=o.collapsedIconSize)!==null&&b!==void 0?b:o.iconSize}),h=I(()=>{if(l.value)return;const{collapsedWidth:b,indent:O,rootIndent:g}=o,{root:_,isGroup:B}=e,N=g===void 0?O:g;return _?n.value?b/2-s.value/2:N:i&&typeof i.paddingLeftRef.value=="number"?O/2+i.paddingLeftRef.value:r&&typeof r.paddingLeftRef.value=="number"?(B?O/2:O)+r.paddingLeftRef.value:0}),p=I(()=>{const{collapsedWidth:b,indent:O,rootIndent:g}=o,{value:_}=s,{root:B}=e;return l.value||!B||!n.value?st:(g===void 0?O:g)+_+st-(b+_)/2});return{dropdownPlacement:a,activeIconSize:c,maxIconSize:s,paddingLeft:h,iconMarginRight:p,NMenu:t,NSubmenu:r,NMenuOptionGroup:i}}const oo={internalKey:{type:[String,Number],required:!0},root:Boolean,isGroup:Boolean,level:{type:Number,required:!0},title:[String,Function],extra:[String,Function]},Ai=re({name:"MenuDivider",setup(){const e=ue(Je),{mergedClsPrefixRef:t,isHorizontalRef:o}=e;return()=>o.value?null:d("div",{class:`${t.value}-menu-divider`})}}),Uo=Object.assign(Object.assign({},oo),{tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function}),Mi=qt(Uo),Bi=re({name:"MenuOption",props:Uo,setup(e){const t=to(e),{NSubmenu:o,NMenu:n,NMenuOptionGroup:r}=t,{props:i,mergedClsPrefixRef:l,mergedCollapsedRef:a}=n,s=o?o.mergedDisabledRef:r?r.mergedDisabledRef:{value:!1},c=I(()=>s.value||e.disabled);function h(b){const{onClick:O}=e;O&&O(b)}function p(b){c.value||(n.doSelect(e.internalKey,e.tmNode.rawNode),h(b))}return{mergedClsPrefix:l,dropdownPlacement:t.dropdownPlacement,paddingLeft:t.paddingLeft,iconMarginRight:t.iconMarginRight,maxIconSize:t.maxIconSize,activeIconSize:t.activeIconSize,mergedTheme:n.mergedThemeRef,menuProps:i,dropdownEnabled:be(()=>e.root&&a.value&&i.mode!=="horizontal"&&!c.value),selected:be(()=>n.mergedValueRef.value===e.internalKey),mergedDisabled:c,handleClick:p}},render(){const{mergedClsPrefix:e,mergedTheme:t,tmNode:o,menuProps:{renderLabel:n,nodeProps:r}}=this,i=r==null?void 0:r(o.rawNode);return d("div",Object.assign({},i,{role:"menuitem",class:[`${e}-menu-item`,i==null?void 0:i.class]}),d(dr,{theme:t.peers.Tooltip,themeOverrides:t.peerOverrides.Tooltip,trigger:"hover",placement:this.dropdownPlacement,disabled:!this.dropdownEnabled||this.title===void 0,internalExtraClass:["menu-tooltip"]},{default:()=>n?n(o.rawNode):he(this.title),trigger:()=>d(Wo,{tmNode:o,clsPrefix:e,paddingLeft:this.paddingLeft,iconMarginRight:this.iconMarginRight,maxIconSize:this.maxIconSize,activeIconSize:this.activeIconSize,selected:this.selected,title:this.title,extra:this.extra,disabled:this.mergedDisabled,icon:this.icon,onClick:this.handleClick})}))}}),Go=Object.assign(Object.assign({},oo),{tmNode:{type:Object,required:!0},tmNodes:{type:Array,required:!0}}),$i=qt(Go),Ei=re({name:"MenuOptionGroup",props:Go,setup(e){const t=to(e),{NSubmenu:o}=t,n=I(()=>o!=null&&o.mergedDisabledRef.value?!0:e.tmNode.disabled);pe(eo,{paddingLeftRef:t.paddingLeft,mergedDisabledRef:n});const{mergedClsPrefixRef:r,props:i}=ue(Je);return function(){const{value:l}=r,a=t.paddingLeft.value,{nodeProps:s}=i,c=s==null?void 0:s(e.tmNode.rawNode);return d("div",{class:`${l}-menu-item-group`,role:"group"},d("div",Object.assign({},c,{class:[`${l}-menu-item-group-title`,c==null?void 0:c.class],style:[(c==null?void 0:c.style)||"",a!==void 0?`padding-left: ${a}px;`:""]}),he(e.title),e.extra?d(Gt,null," ",he(e.extra)):null),d("div",null,e.tmNodes.map(h=>no(h,i))))}}});function Lt(e){return e.type==="divider"||e.type==="render"}function Li(e){return e.type==="divider"}function no(e,t){const{rawNode:o}=e,{show:n}=o;if(n===!1)return null;if(Lt(o))return Li(o)?d(Ai,Object.assign({key:e.key},o.props)):null;const{labelField:r}=t,{key:i,level:l,isGroup:a}=e,s=Object.assign(Object.assign({},o),{title:o.title||o[r],extra:o.titleExtra||o.extra,key:i,internalKey:i,level:l,root:l===0,isGroup:a});return e.children?e.isGroup?d(Ei,dt(s,$i,{tmNode:e,tmNodes:e.children,key:i})):d(Kt,dt(s,Ki,{key:i,rawNodes:o[t.childrenField],tmNodes:e.children,tmNode:e})):d(Bi,dt(s,Mi,{key:i,tmNode:e}))}const qo=Object.assign(Object.assign({},oo),{rawNodes:{type:Array,default:()=>[]},tmNodes:{type:Array,default:()=>[]},tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function,domId:String,virtualChildActive:{type:Boolean,default:void 0},isEllipsisPlaceholder:Boolean}),Ki=qt(qo),Kt=re({name:"Submenu",props:qo,setup(e){const t=to(e),{NMenu:o,NSubmenu:n}=t,{props:r,mergedCollapsedRef:i,mergedThemeRef:l}=o,a=I(()=>{const{disabled:b}=e;return n!=null&&n.mergedDisabledRef.value||r.disabled?!0:b}),s=j(!1);pe(Vo,{paddingLeftRef:t.paddingLeft,mergedDisabledRef:a}),pe(eo,null);function c(){const{onClick:b}=e;b&&b()}function h(){a.value||(i.value||o.toggleExpand(e.internalKey),c())}function p(b){s.value=b}return{menuProps:r,mergedTheme:l,doSelect:o.doSelect,inverted:o.invertedRef,isHorizontal:o.isHorizontalRef,mergedClsPrefix:o.mergedClsPrefixRef,maxIconSize:t.maxIconSize,activeIconSize:t.activeIconSize,iconMarginRight:t.iconMarginRight,dropdownPlacement:t.dropdownPlacement,dropdownShow:s,paddingLeft:t.paddingLeft,mergedDisabled:a,mergedValue:o.mergedValueRef,childActive:be(()=>{var b;return(b=e.virtualChildActive)!==null&&b!==void 0?b:o.activePathRef.value.includes(e.internalKey)}),collapsed:I(()=>r.mode==="horizontal"?!1:i.value?!0:!o.mergedExpandedKeysRef.value.includes(e.internalKey)),dropdownEnabled:I(()=>!a.value&&(r.mode==="horizontal"||i.value)),handlePopoverShowChange:p,handleClick:h}},render(){var e;const{mergedClsPrefix:t,menuProps:{renderIcon:o,renderLabel:n}}=this,r=()=>{const{isHorizontal:l,paddingLeft:a,collapsed:s,mergedDisabled:c,maxIconSize:h,activeIconSize:p,title:b,childActive:O,icon:g,handleClick:_,menuProps:{nodeProps:B},dropdownShow:N,iconMarginRight:K,tmNode:H,mergedClsPrefix:R,isEllipsisPlaceholder:y,extra:T}=this,M=B==null?void 0:B(H.rawNode);return d("div",Object.assign({},M,{class:[`${R}-menu-item`,M==null?void 0:M.class],role:"menuitem"}),d(Wo,{tmNode:H,paddingLeft:a,collapsed:s,disabled:c,iconMarginRight:K,maxIconSize:h,activeIconSize:p,title:b,extra:T,showArrow:!l,childActive:O,clsPrefix:R,icon:g,hover:N,onClick:_,isEllipsisPlaceholder:y}))},i=()=>d(Yn,null,{default:()=>{const{tmNodes:l,collapsed:a}=this;return a?null:d("div",{class:`${t}-submenu-children`,role:"menu"},l.map(s=>no(s,this.menuProps)))}});return this.root?d(xi,Object.assign({size:"large",trigger:"hover"},(e=this.menuProps)===null||e===void 0?void 0:e.dropdownProps,{themeOverrides:this.mergedTheme.peerOverrides.Dropdown,theme:this.mergedTheme.peers.Dropdown,builtinThemeOverrides:{fontSizeLarge:"14px",optionIconSizeLarge:"18px"},value:this.mergedValue,disabled:!this.dropdownEnabled,placement:this.dropdownPlacement,keyField:this.menuProps.keyField,labelField:this.menuProps.labelField,childrenField:this.menuProps.childrenField,onUpdateShow:this.handlePopoverShowChange,options:this.rawNodes,onSelect:this.doSelect,inverted:this.inverted,renderIcon:o,renderLabel:n}),{default:()=>d("div",{class:`${t}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},r(),this.isHorizontal?null:i())}):d("div",{class:`${t}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},r(),i())}}),Hi=Object.assign(Object.assign({},fe.props),{options:{type:Array,default:()=>[]},collapsed:{type:Boolean,default:void 0},collapsedWidth:{type:Number,default:48},iconSize:{type:Number,default:20},collapsedIconSize:{type:Number,default:24},rootIndent:Number,indent:{type:Number,default:32},labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},disabledField:{type:String,default:"disabled"},defaultExpandAll:Boolean,defaultExpandedKeys:Array,expandedKeys:Array,value:[String,Number],defaultValue:{type:[String,Number],default:null},mode:{type:String,default:"vertical"},watchProps:{type:Array,default:void 0},disabled:Boolean,show:{type:Boolean,default:!0},inverted:Boolean,"onUpdate:expandedKeys":[Function,Array],onUpdateExpandedKeys:[Function,Array],onUpdateValue:[Function,Array],"onUpdate:value":[Function,Array],expandIcon:Function,renderIcon:Function,renderLabel:Function,renderExtra:Function,dropdownProps:Object,accordion:Boolean,nodeProps:Function,dropdownPlacement:{type:String,default:"bottom"},responsive:Boolean,items:Array,onOpenNamesChange:[Function,Array],onSelect:[Function,Array],onExpandedNamesChange:[Function,Array],expandedNames:Array,defaultExpandedNames:Array}),Di=re({name:"Menu",inheritAttrs:!1,props:Hi,setup(e){const{mergedClsPrefixRef:t,inlineThemeDisabled:o}=Ie(e),n=fe("Menu","-menu",_i,Zn,e,t),r=ue(Do,null),i=I(()=>{var A;const{collapsed:D}=e;if(D!==void 0)return D;if(r){const{collapseModeRef:v,collapsedRef:f}=r;if(v.value==="width")return(A=f.value)!==null&&A!==void 0?A:!1}return!1}),l=I(()=>{const{keyField:A,childrenField:D,disabledField:v}=e;return Xe(e.items||e.options,{getIgnored(f){return Lt(f)},getChildren(f){return f[D]},getDisabled(f){return f[v]},getKey(f){var w;return(w=f[A])!==null&&w!==void 0?w:f.name}})}),a=I(()=>new Set(l.value.treeNodes.map(A=>A.key))),{watchProps:s}=e,c=j(null);s!=null&&s.includes("defaultValue")?At(()=>{c.value=e.defaultValue}):c.value=e.defaultValue;const h=ie(e,"value"),p=Ke(h,c),b=j([]),O=()=>{b.value=e.defaultExpandAll?l.value.getNonLeafKeys():e.defaultExpandedNames||e.defaultExpandedKeys||l.value.getPath(p.value,{includeSelf:!1}).keyPath};s!=null&&s.includes("defaultExpandedKeys")?At(O):O();const g=To(e,["expandedNames","expandedKeys"]),_=Ke(g,b),B=I(()=>l.value.treeNodes),N=I(()=>l.value.getPath(p.value).keyPath);pe(Je,{props:e,mergedCollapsedRef:i,mergedThemeRef:n,mergedValueRef:p,mergedExpandedKeysRef:_,activePathRef:N,mergedClsPrefixRef:t,isHorizontalRef:I(()=>e.mode==="horizontal"),invertedRef:ie(e,"inverted"),doSelect:K,toggleExpand:R});function K(A,D){const{"onUpdate:value":v,onUpdateValue:f,onSelect:w}=e;f&&ae(f,A,D),v&&ae(v,A,D),w&&ae(w,A,D),c.value=A}function H(A){const{"onUpdate:expandedKeys":D,onUpdateExpandedKeys:v,onExpandedNamesChange:f,onOpenNamesChange:w}=e;D&&ae(D,A),v&&ae(v,A),f&&ae(f,A),w&&ae(w,A),b.value=A}function R(A){const D=Array.from(_.value),v=D.findIndex(f=>f===A);if(~v)D.splice(v,1);else{if(e.accordion&&a.value.has(A)){const f=D.findIndex(w=>a.value.has(w));f>-1&&D.splice(f,1)}D.push(A)}H(D)}const y=A=>{const D=l.value.getPath(A??p.value,{includeSelf:!1}).keyPath;if(!D.length)return;const v=Array.from(_.value),f=new Set([...v,...D]);e.accordion&&a.value.forEach(w=>{f.has(w)&&!D.includes(w)&&f.delete(w)}),H(Array.from(f))},T=I(()=>{const{inverted:A}=e,{common:{cubicBezierEaseInOut:D},self:v}=n.value,{borderRadius:f,borderColorHorizontal:w,fontSize:le,itemHeight:me,dividerColor:we}=v,m={"--n-divider-color":we,"--n-bezier":D,"--n-font-size":le,"--n-border-color-horizontal":w,"--n-border-radius":f,"--n-item-height":me};return A?(m["--n-group-text-color"]=v.groupTextColorInverted,m["--n-color"]=v.colorInverted,m["--n-item-text-color"]=v.itemTextColorInverted,m["--n-item-text-color-hover"]=v.itemTextColorHoverInverted,m["--n-item-text-color-active"]=v.itemTextColorActiveInverted,m["--n-item-text-color-child-active"]=v.itemTextColorChildActiveInverted,m["--n-item-text-color-child-active-hover"]=v.itemTextColorChildActiveInverted,m["--n-item-text-color-active-hover"]=v.itemTextColorActiveHoverInverted,m["--n-item-icon-color"]=v.itemIconColorInverted,m["--n-item-icon-color-hover"]=v.itemIconColorHoverInverted,m["--n-item-icon-color-active"]=v.itemIconColorActiveInverted,m["--n-item-icon-color-active-hover"]=v.itemIconColorActiveHoverInverted,m["--n-item-icon-color-child-active"]=v.itemIconColorChildActiveInverted,m["--n-item-icon-color-child-active-hover"]=v.itemIconColorChildActiveHoverInverted,m["--n-item-icon-color-collapsed"]=v.itemIconColorCollapsedInverted,m["--n-item-text-color-horizontal"]=v.itemTextColorHorizontalInverted,m["--n-item-text-color-hover-horizontal"]=v.itemTextColorHoverHorizontalInverted,m["--n-item-text-color-active-horizontal"]=v.itemTextColorActiveHorizontalInverted,m["--n-item-text-color-child-active-horizontal"]=v.itemTextColorChildActiveHorizontalInverted,m["--n-item-text-color-child-active-hover-horizontal"]=v.itemTextColorChildActiveHoverHorizontalInverted,m["--n-item-text-color-active-hover-horizontal"]=v.itemTextColorActiveHoverHorizontalInverted,m["--n-item-icon-color-horizontal"]=v.itemIconColorHorizontalInverted,m["--n-item-icon-color-hover-horizontal"]=v.itemIconColorHoverHorizontalInverted,m["--n-item-icon-color-active-horizontal"]=v.itemIconColorActiveHorizontalInverted,m["--n-item-icon-color-active-hover-horizontal"]=v.itemIconColorActiveHoverHorizontalInverted,m["--n-item-icon-color-child-active-horizontal"]=v.itemIconColorChildActiveHorizontalInverted,m["--n-item-icon-color-child-active-hover-horizontal"]=v.itemIconColorChildActiveHoverHorizontalInverted,m["--n-arrow-color"]=v.arrowColorInverted,m["--n-arrow-color-hover"]=v.arrowColorHoverInverted,m["--n-arrow-color-active"]=v.arrowColorActiveInverted,m["--n-arrow-color-active-hover"]=v.arrowColorActiveHoverInverted,m["--n-arrow-color-child-active"]=v.arrowColorChildActiveInverted,m["--n-arrow-color-child-active-hover"]=v.arrowColorChildActiveHoverInverted,m["--n-item-color-hover"]=v.itemColorHoverInverted,m["--n-item-color-active"]=v.itemColorActiveInverted,m["--n-item-color-active-hover"]=v.itemColorActiveHoverInverted,m["--n-item-color-active-collapsed"]=v.itemColorActiveCollapsedInverted):(m["--n-group-text-color"]=v.groupTextColor,m["--n-color"]=v.color,m["--n-item-text-color"]=v.itemTextColor,m["--n-item-text-color-hover"]=v.itemTextColorHover,m["--n-item-text-color-active"]=v.itemTextColorActive,m["--n-item-text-color-child-active"]=v.itemTextColorChildActive,m["--n-item-text-color-child-active-hover"]=v.itemTextColorChildActiveHover,m["--n-item-text-color-active-hover"]=v.itemTextColorActiveHover,m["--n-item-icon-color"]=v.itemIconColor,m["--n-item-icon-color-hover"]=v.itemIconColorHover,m["--n-item-icon-color-active"]=v.itemIconColorActive,m["--n-item-icon-color-active-hover"]=v.itemIconColorActiveHover,m["--n-item-icon-color-child-active"]=v.itemIconColorChildActive,m["--n-item-icon-color-child-active-hover"]=v.itemIconColorChildActiveHover,m["--n-item-icon-color-collapsed"]=v.itemIconColorCollapsed,m["--n-item-text-color-horizontal"]=v.itemTextColorHorizontal,m["--n-item-text-color-hover-horizontal"]=v.itemTextColorHoverHorizontal,m["--n-item-text-color-active-horizontal"]=v.itemTextColorActiveHorizontal,m["--n-item-text-color-child-active-horizontal"]=v.itemTextColorChildActiveHorizontal,m["--n-item-text-color-child-active-hover-horizontal"]=v.itemTextColorChildActiveHoverHorizontal,m["--n-item-text-color-active-hover-horizontal"]=v.itemTextColorActiveHoverHorizontal,m["--n-item-icon-color-horizontal"]=v.itemIconColorHorizontal,m["--n-item-icon-color-hover-horizontal"]=v.itemIconColorHoverHorizontal,m["--n-item-icon-color-active-horizontal"]=v.itemIconColorActiveHorizontal,m["--n-item-icon-color-active-hover-horizontal"]=v.itemIconColorActiveHoverHorizontal,m["--n-item-icon-color-child-active-horizontal"]=v.itemIconColorChildActiveHorizontal,m["--n-item-icon-color-child-active-hover-horizontal"]=v.itemIconColorChildActiveHoverHorizontal,m["--n-arrow-color"]=v.arrowColor,m["--n-arrow-color-hover"]=v.arrowColorHover,m["--n-arrow-color-active"]=v.arrowColorActive,m["--n-arrow-color-active-hover"]=v.arrowColorActiveHover,m["--n-arrow-color-child-active"]=v.arrowColorChildActive,m["--n-arrow-color-child-active-hover"]=v.arrowColorChildActiveHover,m["--n-item-color-hover"]=v.itemColorHover,m["--n-item-color-active"]=v.itemColorActive,m["--n-item-color-active-hover"]=v.itemColorActiveHover,m["--n-item-color-active-collapsed"]=v.itemColorActiveCollapsed),m}),M=o?Pe("menu",I(()=>e.inverted?"a":"b"),T,e):void 0,Y=Xn(),te=j(null),J=j(null);let Z=!0;const se=()=>{var A;Z?Z=!1:(A=te.value)===null||A===void 0||A.sync({showAllItemsBeforeCalculate:!0})};function C(){return document.getElementById(Y)}const x=j(-1);function $(A){x.value=e.options.length-A}function L(A){A||(x.value=-1)}const P=I(()=>{const A=x.value;return{children:A===-1?[]:e.options.slice(A)}}),V=I(()=>{const{childrenField:A,disabledField:D,keyField:v}=e;return Xe([P.value],{getIgnored(f){return Lt(f)},getChildren(f){return f[A]},getDisabled(f){return f[D]},getKey(f){var w;return(w=f[v])!==null&&w!==void 0?w:f.name}})}),q=I(()=>Xe([{}]).treeNodes[0]);function ne(){var A;if(x.value===-1)return d(Kt,{root:!0,level:0,key:"__ellpisisGroupPlaceholder__",internalKey:"__ellpisisGroupPlaceholder__",title:"···",tmNode:q.value,domId:Y,isEllipsisPlaceholder:!0});const D=V.value.treeNodes[0],v=N.value,f=!!(!((A=D.children)===null||A===void 0)&&A.some(w=>v.includes(w.key)));return d(Kt,{level:0,root:!0,key:"__ellpisisGroup__",internalKey:"__ellpisisGroup__",title:"···",virtualChildActive:f,tmNode:D,domId:Y,rawNodes:D.rawNode.children||[],tmNodes:D.children||[],isEllipsisPlaceholder:!0})}return{mergedClsPrefix:t,controlledExpandedKeys:g,uncontrolledExpanededKeys:b,mergedExpandedKeys:_,uncontrolledValue:c,mergedValue:p,activePath:N,tmNodes:B,mergedTheme:n,mergedCollapsed:i,cssVars:o?void 0:T,themeClass:M==null?void 0:M.themeClass,overflowRef:te,counterRef:J,updateCounter:()=>{},onResize:se,onUpdateOverflow:L,onUpdateCount:$,renderCounter:ne,getCounter:C,onRender:M==null?void 0:M.onRender,showOption:y,deriveResponsiveState:se}},render(){const{mergedClsPrefix:e,mode:t,themeClass:o,onRender:n}=this;n==null||n();const r=()=>this.tmNodes.map(s=>no(s,this.$props)),l=t==="horizontal"&&this.responsive,a=()=>d("div",Ze(this.$attrs,{role:t==="horizontal"?"menubar":"menu",class:[`${e}-menu`,o,`${e}-menu--${t}`,l&&`${e}-menu--responsive`,this.mergedCollapsed&&`${e}-menu--collapsed`],style:this.cssVars}),l?d(Bt,{ref:"overflowRef",onUpdateOverflow:this.onUpdateOverflow,getCounter:this.getCounter,onUpdateCount:this.onUpdateCount,updateCounter:this.updateCounter,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:r,counter:this.renderCounter}):r());return l?d(Ft,{onResize:this.onResize},{default:a}):a()}}),ji={class:"left"},Vi={class:"right"},Wi={class:"who"},Ui=re({__name:"Shell",setup(e){const t=Jn(),o=cr(),n=nr(),r=rr(),i=vr(),l=[{label:"账号与直播间",key:"accounts"},{label:"房管",key:"moderation"},{label:"弹幕姬",key:"danmaku"},{label:"自定义弹幕姬",key:"custom"},{label:"统计",key:"stats"},{label:"日志",key:"logs"},{label:"管理",key:"admin"}],a=I(()=>o.list.map(h=>({label:`${h.accountName} @ ${h.roomId}${h.enabled?"":"（已停用）"}`,value:h.id})));He(()=>void o.refresh());function s(h){if(!r.hasRoute(h)){i.info("这个页面还没做");return}r.push({name:h})}function c(){t.logout().catch(()=>{}).finally(()=>r.push("/login"))}return(h,p)=>(Ct(),wt(ve(Nt),{"has-sider":"",position:"absolute"},{default:Le(()=>[_e(ve(Fi),{bordered:"",width:180,"content-style":"padding-top: 12px"},{default:Le(()=>[_e(ve(Di),{value:String(ve(n).name),options:l,"onUpdate:value":s},null,8,["value"])]),_:1}),_e(ve(Nt),null,{default:Le(()=>[_e(ve(Pi),{bordered:"",class:"header"},{default:Le(()=>{var b;return[St("div",ji,[ve(o).loading?(Ct(),wt(ve(ur),{key:0,size:"small"})):(Ct(),wt(ve(ai),{key:1,value:ve(o).currentId,options:a.value,placeholder:"没有可用的直播间",style:{width:"260px"},"onUpdate:value":ve(o).select},null,8,["value","options","onUpdate:value"]))]),St("div",Vi,[St("span",Wi,Qn((b=ve(t).user)==null?void 0:b.username),1),_e(ve(er),{text:"",size:"small",onClick:c},{default:Le(()=>[...p[0]||(p[0]=[tr(" 退出 ",-1)])]),_:1})])]}),_:1}),_e(ve(Nt),{"content-style":"padding: 16px"},{default:Le(()=>[_e(ve(or))]),_:1})]),_:1})]),_:1}))}}),Xi=pr(Ui,[["__scopeId","data-v-c0865bc2"]]);export{Xi as default};
