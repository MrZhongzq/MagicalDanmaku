import{n as b,e as w,h as v,a as I,m as k,d as P,W as ve,u as A,f as $,x as K,cu as ge,A as C,R as pe,cv as Ce,b7 as h,b as x,Y as H,q as Y,be,as as fe,r as R,X as me,bj as ye,cw as q,p as xe,t as ke,c as ze,cx as Se,bb as Ie,T as Pe,a4 as Re,cy as $e,cz as Be,bc as He,cA as we,ad as Te,O as Ee}from"./index-BNvL3Drp.js";import{a as _e}from"./_plugin-vue_export-helper-0GGlbz9E.js";function Oe(e,r){return b(()=>{for(const o of r)if(e[o]!==void 0)return e[o];return e[r[r.length-1]]})}const Me=w({name:"Empty",render(){return v("svg",{viewBox:"0 0 28 28",fill:"none",xmlns:"http://www.w3.org/2000/svg"},v("path",{d:"M26 7.5C26 11.0899 23.0899 14 19.5 14C15.9101 14 13 11.0899 13 7.5C13 3.91015 15.9101 1 19.5 1C23.0899 1 26 3.91015 26 7.5ZM16.8536 4.14645C16.6583 3.95118 16.3417 3.95118 16.1464 4.14645C15.9512 4.34171 15.9512 4.65829 16.1464 4.85355L18.7929 7.5L16.1464 10.1464C15.9512 10.3417 15.9512 10.6583 16.1464 10.8536C16.3417 11.0488 16.6583 11.0488 16.8536 10.8536L19.5 8.20711L22.1464 10.8536C22.3417 11.0488 22.6583 11.0488 22.8536 10.8536C23.0488 10.6583 23.0488 10.3417 22.8536 10.1464L20.2071 7.5L22.8536 4.85355C23.0488 4.65829 23.0488 4.34171 22.8536 4.14645C22.6583 3.95118 22.3417 3.95118 22.1464 4.14645L19.5 6.79289L16.8536 4.14645Z",fill:"currentColor"}),v("path",{d:"M25 22.75V12.5991C24.5572 13.0765 24.053 13.4961 23.5 13.8454V16H17.5L17.3982 16.0068C17.0322 16.0565 16.75 16.3703 16.75 16.75C16.75 18.2688 15.5188 19.5 14 19.5C12.4812 19.5 11.25 18.2688 11.25 16.75L11.2432 16.6482C11.1935 16.2822 10.8797 16 10.5 16H4.5V7.25C4.5 6.2835 5.2835 5.5 6.25 5.5H12.2696C12.4146 4.97463 12.6153 4.47237 12.865 4H6.25C4.45507 4 3 5.45507 3 7.25V22.75C3 24.5449 4.45507 26 6.25 26H21.75C23.5449 26 25 24.5449 25 22.75ZM4.5 22.75V17.5H9.81597L9.85751 17.7041C10.2905 19.5919 11.9808 21 14 21L14.215 20.9947C16.2095 20.8953 17.842 19.4209 18.184 17.5H23.5V22.75C23.5 23.7165 22.7165 24.5 21.75 24.5H6.25C5.2835 24.5 4.5 23.7165 4.5 22.75Z",fill:"currentColor"}))}}),je=I("empty",`
 display: flex;
 flex-direction: column;
 align-items: center;
 font-size: var(--n-font-size);
`,[k("icon",`
 width: var(--n-icon-size);
 height: var(--n-icon-size);
 font-size: var(--n-icon-size);
 line-height: var(--n-icon-size);
 color: var(--n-icon-color);
 transition:
 color .3s var(--n-bezier);
 `,[P("+",[k("description",`
 margin-top: 8px;
 `)])]),k("description",`
 transition: color .3s var(--n-bezier);
 color: var(--n-text-color);
 `),k("extra",`
 text-align: center;
 transition: color .3s var(--n-bezier);
 margin-top: 12px;
 color: var(--n-extra-text-color);
 `)]),Le=Object.assign(Object.assign({},$.props),{description:String,showDescription:{type:Boolean,default:!0},showIcon:{type:Boolean,default:!0},size:{type:String,default:"medium"},renderIcon:Function}),Ge=w({name:"Empty",props:Le,slots:Object,setup(e){const{mergedClsPrefixRef:r,inlineThemeDisabled:o,mergedComponentPropsRef:l}=A(e),i=$("Empty","-empty",je,ge,e,r),{localeRef:c}=_e("Empty"),d=b(()=>{var a,u,y;return(a=e.description)!==null&&a!==void 0?a:(y=(u=l==null?void 0:l.value)===null||u===void 0?void 0:u.Empty)===null||y===void 0?void 0:y.description}),t=b(()=>{var a,u;return((u=(a=l==null?void 0:l.value)===null||a===void 0?void 0:a.Empty)===null||u===void 0?void 0:u.renderIcon)||(()=>v(Me,null))}),n=b(()=>{const{size:a}=e,{common:{cubicBezierEaseInOut:u},self:{[C("iconSize",a)]:y,[C("fontSize",a)]:S,textColor:f,iconColor:s,extraTextColor:p}}=i.value;return{"--n-icon-size":y,"--n-font-size":S,"--n-bezier":u,"--n-text-color":f,"--n-icon-color":s,"--n-extra-text-color":p}}),g=o?K("empty",b(()=>{let a="";const{size:u}=e;return a+=u[0],a}),n,e):void 0;return{mergedClsPrefix:r,mergedRenderIcon:t,localizedDescription:b(()=>d.value||c.value.description),cssVars:o?void 0:n,themeClass:g==null?void 0:g.themeClass,onRender:g==null?void 0:g.onRender}},render(){const{$slots:e,mergedClsPrefix:r,onRender:o}=this;return o==null||o(),v("div",{class:[`${r}-empty`,this.themeClass],style:this.cssVars},this.showIcon?v("div",{class:`${r}-empty__icon`},e.icon?e.icon():v(ve,{clsPrefix:r},{default:this.mergedRenderIcon})):null,this.showDescription?v("div",{class:`${r}-empty__description`},e.default?e.default():this.localizedDescription):null,e.extra?v("div",{class:`${r}-empty__extra`},e.extra()):null)}});function Ve(e){const{textColor2:r,primaryColorHover:o,primaryColorPressed:l,primaryColor:i,infoColor:c,successColor:d,warningColor:t,errorColor:n,baseColor:g,borderColor:a,opacityDisabled:u,tagColor:y,closeIconColor:S,closeIconColorHover:f,closeIconColorPressed:s,borderRadiusSmall:p,fontSizeMini:z,fontSizeTiny:m,fontSizeSmall:T,fontSizeMedium:E,heightMini:_,heightTiny:O,heightSmall:M,heightMedium:j,closeColorHover:L,closeColorPressed:V,buttonColor2Hover:W,buttonColor2Pressed:N,fontWeightStrong:D}=e;return Object.assign(Object.assign({},Ce),{closeBorderRadius:p,heightTiny:_,heightSmall:O,heightMedium:M,heightLarge:j,borderRadius:p,opacityDisabled:u,fontSizeTiny:z,fontSizeSmall:m,fontSizeMedium:T,fontSizeLarge:E,fontWeightStrong:D,textColorCheckable:r,textColorHoverCheckable:r,textColorPressedCheckable:r,textColorChecked:g,colorCheckable:"#0000",colorHoverCheckable:W,colorPressedCheckable:N,colorChecked:i,colorCheckedHover:o,colorCheckedPressed:l,border:`1px solid ${a}`,textColor:r,color:y,colorBordered:"rgb(250, 250, 252)",closeIconColor:S,closeIconColorHover:f,closeIconColorPressed:s,closeColorHover:L,closeColorPressed:V,borderPrimary:`1px solid ${h(i,{alpha:.3})}`,textColorPrimary:i,colorPrimary:h(i,{alpha:.12}),colorBorderedPrimary:h(i,{alpha:.1}),closeIconColorPrimary:i,closeIconColorHoverPrimary:i,closeIconColorPressedPrimary:i,closeColorHoverPrimary:h(i,{alpha:.12}),closeColorPressedPrimary:h(i,{alpha:.18}),borderInfo:`1px solid ${h(c,{alpha:.3})}`,textColorInfo:c,colorInfo:h(c,{alpha:.12}),colorBorderedInfo:h(c,{alpha:.1}),closeIconColorInfo:c,closeIconColorHoverInfo:c,closeIconColorPressedInfo:c,closeColorHoverInfo:h(c,{alpha:.12}),closeColorPressedInfo:h(c,{alpha:.18}),borderSuccess:`1px solid ${h(d,{alpha:.3})}`,textColorSuccess:d,colorSuccess:h(d,{alpha:.12}),colorBorderedSuccess:h(d,{alpha:.1}),closeIconColorSuccess:d,closeIconColorHoverSuccess:d,closeIconColorPressedSuccess:d,closeColorHoverSuccess:h(d,{alpha:.12}),closeColorPressedSuccess:h(d,{alpha:.18}),borderWarning:`1px solid ${h(t,{alpha:.35})}`,textColorWarning:t,colorWarning:h(t,{alpha:.15}),colorBorderedWarning:h(t,{alpha:.12}),closeIconColorWarning:t,closeIconColorHoverWarning:t,closeIconColorPressedWarning:t,closeColorHoverWarning:h(t,{alpha:.12}),closeColorPressedWarning:h(t,{alpha:.18}),borderError:`1px solid ${h(n,{alpha:.23})}`,textColorError:n,colorError:h(n,{alpha:.1}),colorBorderedError:h(n,{alpha:.08}),closeIconColorError:n,closeIconColorHoverError:n,closeIconColorPressedError:n,closeColorHoverError:h(n,{alpha:.12}),closeColorPressedError:h(n,{alpha:.18})})}const We={name:"Tag",common:pe,self:Ve},Ne={color:Object,type:{type:String,default:"default"},round:Boolean,size:String,closable:Boolean,disabled:{type:Boolean,default:void 0}},De=I("tag",`
 --n-close-margin: var(--n-close-margin-top) var(--n-close-margin-right) var(--n-close-margin-bottom) var(--n-close-margin-left);
 white-space: nowrap;
 position: relative;
 box-sizing: border-box;
 cursor: default;
 display: inline-flex;
 align-items: center;
 flex-wrap: nowrap;
 padding: var(--n-padding);
 border-radius: var(--n-border-radius);
 color: var(--n-text-color);
 background-color: var(--n-color);
 transition: 
 border-color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 line-height: 1;
 height: var(--n-height);
 font-size: var(--n-font-size);
`,[x("strong",`
 font-weight: var(--n-font-weight-strong);
 `),k("border",`
 pointer-events: none;
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 border-radius: inherit;
 border: var(--n-border);
 transition: border-color .3s var(--n-bezier);
 `),k("icon",`
 display: flex;
 margin: 0 4px 0 0;
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 font-size: var(--n-avatar-size-override);
 `),k("avatar",`
 display: flex;
 margin: 0 6px 0 0;
 `),k("close",`
 margin: var(--n-close-margin);
 transition:
 background-color .3s var(--n-bezier),
 color .3s var(--n-bezier);
 `),x("round",`
 padding: 0 calc(var(--n-height) / 3);
 border-radius: calc(var(--n-height) / 2);
 `,[k("icon",`
 margin: 0 4px 0 calc((var(--n-height) - 8px) / -2);
 `),k("avatar",`
 margin: 0 6px 0 calc((var(--n-height) - 8px) / -2);
 `),x("closable",`
 padding: 0 calc(var(--n-height) / 4) 0 calc(var(--n-height) / 3);
 `)]),x("icon, avatar",[x("round",`
 padding: 0 calc(var(--n-height) / 3) 0 calc(var(--n-height) / 2);
 `)]),x("disabled",`
 cursor: not-allowed !important;
 opacity: var(--n-opacity-disabled);
 `),x("checkable",`
 cursor: pointer;
 box-shadow: none;
 color: var(--n-text-color-checkable);
 background-color: var(--n-color-checkable);
 `,[H("disabled",[P("&:hover","background-color: var(--n-color-hover-checkable);",[H("checked","color: var(--n-text-color-hover-checkable);")]),P("&:active","background-color: var(--n-color-pressed-checkable);",[H("checked","color: var(--n-text-color-pressed-checkable);")])]),x("checked",`
 color: var(--n-text-color-checked);
 background-color: var(--n-color-checked);
 `,[H("disabled",[P("&:hover","background-color: var(--n-color-checked-hover);"),P("&:active","background-color: var(--n-color-checked-pressed);")])])])]),Fe=Object.assign(Object.assign(Object.assign({},$.props),Ne),{bordered:{type:Boolean,default:void 0},checked:Boolean,checkable:Boolean,strong:Boolean,triggerClickOnClose:Boolean,onClose:[Array,Function],onMouseenter:Function,onMouseleave:Function,"onUpdate:checked":Function,onUpdateChecked:Function,internalCloseFocusable:{type:Boolean,default:!0},internalCloseIsButtonTag:{type:Boolean,default:!0},onCheckedChange:Function}),Ae=ze("n-tag"),Xe=w({name:"Tag",props:Fe,slots:Object,setup(e){const r=R(null),{mergedBorderedRef:o,mergedClsPrefixRef:l,inlineThemeDisabled:i,mergedRtlRef:c,mergedComponentPropsRef:d}=A(e),t=b(()=>{var s,p;return e.size||((p=(s=d==null?void 0:d.value)===null||s===void 0?void 0:s.Tag)===null||p===void 0?void 0:p.size)||"medium"}),n=$("Tag","-tag",De,We,e,l);xe(Ae,{roundRef:ke(e,"round")});function g(){if(!e.disabled&&e.checkable){const{checked:s,onCheckedChange:p,onUpdateChecked:z,"onUpdate:checked":m}=e;z&&z(!s),m&&m(!s),p&&p(!s)}}function a(s){if(e.triggerClickOnClose||s.stopPropagation(),!e.disabled){const{onClose:p}=e;p&&me(p,s)}}const u={setTextContent(s){const{value:p}=r;p&&(p.textContent=s)}},y=fe("Tag",c,l),S=b(()=>{const{type:s,color:{color:p,textColor:z}={}}=e,m=t.value,{common:{cubicBezierEaseInOut:T},self:{padding:E,closeMargin:_,borderRadius:O,opacityDisabled:M,textColorCheckable:j,textColorHoverCheckable:L,textColorPressedCheckable:V,textColorChecked:W,colorCheckable:N,colorHoverCheckable:D,colorPressedCheckable:G,colorChecked:X,colorCheckedHover:J,colorCheckedPressed:Q,closeBorderRadius:ee,fontWeightStrong:oe,[C("colorBordered",s)]:re,[C("closeSize",m)]:ne,[C("closeIconSize",m)]:te,[C("fontSize",m)]:se,[C("height",m)]:U,[C("color",s)]:le,[C("textColor",s)]:ae,[C("border",s)]:ie,[C("closeIconColor",s)]:Z,[C("closeIconColorHover",s)]:ce,[C("closeIconColorPressed",s)]:de,[C("closeColorHover",s)]:he,[C("closeColorPressed",s)]:ue}}=n.value,B=ye(_);return{"--n-font-weight-strong":oe,"--n-avatar-size-override":`calc(${U} - 8px)`,"--n-bezier":T,"--n-border-radius":O,"--n-border":ie,"--n-close-icon-size":te,"--n-close-color-pressed":ue,"--n-close-color-hover":he,"--n-close-border-radius":ee,"--n-close-icon-color":Z,"--n-close-icon-color-hover":ce,"--n-close-icon-color-pressed":de,"--n-close-icon-color-disabled":Z,"--n-close-margin-top":B.top,"--n-close-margin-right":B.right,"--n-close-margin-bottom":B.bottom,"--n-close-margin-left":B.left,"--n-close-size":ne,"--n-color":p||(o.value?re:le),"--n-color-checkable":N,"--n-color-checked":X,"--n-color-checked-hover":J,"--n-color-checked-pressed":Q,"--n-color-hover-checkable":D,"--n-color-pressed-checkable":G,"--n-font-size":se,"--n-height":U,"--n-opacity-disabled":M,"--n-padding":E,"--n-text-color":z||ae,"--n-text-color-checkable":j,"--n-text-color-checked":W,"--n-text-color-hover-checkable":L,"--n-text-color-pressed-checkable":V}}),f=i?K("tag",b(()=>{let s="";const{type:p,color:{color:z,textColor:m}={}}=e;return s+=p[0],s+=t.value[0],z&&(s+=`a${q(z)}`),m&&(s+=`b${q(m)}`),o.value&&(s+="c"),s}),S,e):void 0;return Object.assign(Object.assign({},u),{rtlEnabled:y,mergedClsPrefix:l,contentRef:r,mergedBordered:o,handleClick:g,handleCloseClick:a,cssVars:i?void 0:S,themeClass:f==null?void 0:f.themeClass,onRender:f==null?void 0:f.onRender})},render(){var e,r;const{mergedClsPrefix:o,rtlEnabled:l,closable:i,color:{borderColor:c}={},round:d,onRender:t,$slots:n}=this;t==null||t();const g=Y(n.avatar,u=>u&&v("div",{class:`${o}-tag__avatar`},u)),a=Y(n.icon,u=>u&&v("div",{class:`${o}-tag__icon`},u));return v("div",{class:[`${o}-tag`,this.themeClass,{[`${o}-tag--rtl`]:l,[`${o}-tag--strong`]:this.strong,[`${o}-tag--disabled`]:this.disabled,[`${o}-tag--checkable`]:this.checkable,[`${o}-tag--checked`]:this.checkable&&this.checked,[`${o}-tag--round`]:d,[`${o}-tag--avatar`]:g,[`${o}-tag--icon`]:a,[`${o}-tag--closable`]:i}],style:this.cssVars,onClick:this.handleClick,onMouseenter:this.onMouseenter,onMouseleave:this.onMouseleave},a||g,v("span",{class:`${o}-tag__content`,ref:"contentRef"},(r=(e=this.$slots).default)===null||r===void 0?void 0:r.call(e)),!this.checkable&&i?v(be,{clsPrefix:o,class:`${o}-tag__close`,disabled:this.disabled,onClick:this.handleCloseClick,focusable:this.internalCloseFocusable,round:d,isButtonTag:this.internalCloseIsButtonTag,absolute:!0}):null,!this.checkable&&this.mergedBordered?v("div",{class:`${o}-tag__border`,style:{borderColor:c}}):null)}}),Ke=P([P("@keyframes spin-rotate",`
 from {
 transform: rotate(0);
 }
 to {
 transform: rotate(360deg);
 }
 `),I("spin-container",`
 position: relative;
 `,[I("spin-body",`
 position: absolute;
 top: 50%;
 left: 50%;
 transform: translateX(-50%) translateY(-50%);
 `,[Se()])]),I("spin-body",`
 display: inline-flex;
 align-items: center;
 justify-content: center;
 flex-direction: column;
 `),I("spin",`
 display: inline-flex;
 height: var(--n-size);
 width: var(--n-size);
 font-size: var(--n-size);
 color: var(--n-color);
 `,[x("rotate",`
 animation: spin-rotate 2s linear infinite;
 `)]),I("spin-description",`
 display: inline-block;
 font-size: var(--n-font-size);
 color: var(--n-text-color);
 transition: color .3s var(--n-bezier);
 margin-top: 8px;
 `),I("spin-content",`
 opacity: 1;
 transition: opacity .3s var(--n-bezier);
 pointer-events: all;
 `,[x("spinning",`
 user-select: none;
 -webkit-user-select: none;
 pointer-events: none;
 opacity: var(--n-opacity-spinning);
 `)])]),Ue={small:20,medium:18,large:16},Ze=Object.assign(Object.assign(Object.assign({},$.props),{contentClass:String,contentStyle:[Object,String],description:String,size:{type:[String,Number],default:"medium"},show:{type:Boolean,default:!0},rotate:{type:Boolean,default:!0},spinning:{type:Boolean,validator:()=>!0,default:void 0},delay:Number}),$e),Je=w({name:"Spin",props:Ze,slots:Object,setup(e){const{mergedClsPrefixRef:r,inlineThemeDisabled:o}=A(e),l=$("Spin","-spin",Ke,Be,e,r),i=b(()=>{const{size:n}=e,{common:{cubicBezierEaseInOut:g},self:a}=l.value,{opacitySpinning:u,color:y,textColor:S}=a,f=typeof n=="number"?He(n):a[C("size",n)];return{"--n-bezier":g,"--n-opacity-spinning":u,"--n-size":f,"--n-color":y,"--n-text-color":S}}),c=o?K("spin",b(()=>{const{size:n}=e;return typeof n=="number"?String(n):n[0]}),i,e):void 0,d=Oe(e,["spinning","show"]),t=R(!1);return Re(n=>{let g;if(d.value){const{delay:a}=e;if(a){g=window.setTimeout(()=>{t.value=!0},a),n(()=>{clearTimeout(g)});return}}t.value=d.value}),{mergedClsPrefix:r,active:t,mergedStrokeWidth:b(()=>{const{strokeWidth:n}=e;if(n!==void 0)return n;const{size:g}=e;return Ue[typeof g=="number"?"medium":g]}),cssVars:o?void 0:i,themeClass:c==null?void 0:c.themeClass,onRender:c==null?void 0:c.onRender}},render(){var e,r;const{$slots:o,mergedClsPrefix:l,description:i}=this,c=o.icon&&this.rotate,d=(i||o.description)&&v("div",{class:`${l}-spin-description`},i||((e=o.description)===null||e===void 0?void 0:e.call(o))),t=o.icon?v("div",{class:[`${l}-spin-body`,this.themeClass]},v("div",{class:[`${l}-spin`,c&&`${l}-spin--rotate`],style:o.default?"":this.cssVars},o.icon()),d):v("div",{class:[`${l}-spin-body`,this.themeClass]},v(Ie,{clsPrefix:l,style:o.default?"":this.cssVars,stroke:this.stroke,"stroke-width":this.mergedStrokeWidth,radius:this.radius,scale:this.scale,class:`${l}-spin`}),d);return(r=this.onRender)===null||r===void 0||r.call(this),o.default?v("div",{class:[`${l}-spin-container`,this.themeClass],style:this.cssVars},v("div",{class:[`${l}-spin-content`,this.active&&`${l}-spin-content--spinning`,this.contentClass],style:this.contentStyle},o),v(Pe,{name:"fade-in-transition"},{default:()=>this.active?t:null})):t}}),F="magicd.currentBinding",Qe=we("bindings",()=>{const e=R([]),r=R(null),o=R(!1),l=R(null),i=b(()=>e.value.find(t=>t.id===r.value)??null);function c(t){r.value=t,localStorage.setItem(F,String(t))}async function d(){var t;o.value=!0,l.value=null;try{e.value=await Te("GET","/api/bindings");const n=Number(localStorage.getItem(F));e.value.some(a=>a.id===n)?r.value=n:(r.value=((t=e.value[0])==null?void 0:t.id)??null,r.value!==null&&localStorage.setItem(F,String(r.value)))}catch(n){l.value=n instanceof Ee?n.message:"加载直播间列表失败",e.value=[]}finally{o.value=!1}}return{list:e,current:i,currentId:r,loading:o,loadError:l,select:c,refresh:d}});export{Je as N,Qe as a,Ge as b,Xe as c,Ne as d,We as t,Oe as u};
