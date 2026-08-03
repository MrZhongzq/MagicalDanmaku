import{e as E,h as d,P as Ze,Q as Qe,R as Je,S as Se,c as Q,a as u,b as S,U as ke,u as ie,f as U,V as _e,x as le,n as b,r as F,p as Y,m as s,d as z,W as He,i as D,X as $,t as ne,Y as X,Z as eo,_ as q,k as ge,$ as ve,a0 as oo,a1 as ce,a2 as to,a3 as ro,a4 as we,s as no,a5 as io,a6 as lo,B as ao,v as co,a7 as oe,E as K,F as _,H as te,D as V,M as Z,a8 as Re,K as Pe,L as Ne,a9 as so,aa as uo,ab as vo,G as mo}from"./index-M2ANDU26.js";import{u as ho,a as fo,N as go}from"./bindings-CtyK8z4P.js";import{C as po,N as bo,V as xo,c as se,a as Co}from"./Tooltip-DMmrCU1p.js";import{N as yo}from"./Dropdown-DIEnAMGn.js";import{u as me,_ as zo}from"./_plugin-vue_export-helper-etY_g7q3.js";import{f as de}from"./get-C62Udozn.js";import{N as Io}from"./Alert-DZEKi4x6.js";import{u as So}from"./use-message-GjlxM5f3.js";const wo=E({name:"ChevronDownFilled",render(){return d("svg",{viewBox:"0 0 16 16",fill:"none",xmlns:"http://www.w3.org/2000/svg"},d("path",{d:"M3.20041 5.73966C3.48226 5.43613 3.95681 5.41856 4.26034 5.70041L8 9.22652L11.7397 5.70041C12.0432 5.41856 12.5177 5.43613 12.7996 5.73966C13.0815 6.0432 13.0639 6.51775 12.7603 6.7996L8.51034 10.7996C8.22258 11.0668 7.77743 11.0668 7.48967 10.7996L3.23966 6.7996C2.93613 6.51775 2.91856 6.0432 3.20041 5.73966Z",fill:"currentColor"}))}});function Ro(e){const{baseColor:r,textColor2:o,bodyColor:a,cardColor:l,dividerColor:i,actionColor:m,scrollbarColor:f,scrollbarColorHover:c,invertedColor:p}=e;return{textColor:o,textColorInverted:"#FFF",color:a,colorEmbedded:m,headerColor:l,headerColorInverted:p,footerColor:m,footerColorInverted:p,headerBorderColor:i,headerBorderColorInverted:p,footerBorderColor:i,footerBorderColorInverted:p,siderBorderColor:i,siderBorderColorInverted:p,siderColor:l,siderColorInverted:p,siderToggleButtonBorder:`1px solid ${i}`,siderToggleButtonColor:r,siderToggleButtonIconColor:o,siderToggleButtonIconColorInverted:o,siderToggleBarColor:Se(a,f),siderToggleBarColorHover:Se(a,c),__invertScrollbar:"true"}}const pe=Ze({name:"Layout",common:Je,peers:{Scrollbar:Qe},self:Ro}),Be=Q("n-layout-sider"),be={type:String,default:"static"},Po=u("layout",`
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
`,[u("layout-scroll-container",`
 overflow-x: hidden;
 box-sizing: border-box;
 height: 100%;
 `),S("absolute-positioned",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),No={embedded:Boolean,position:be,nativeScrollbar:{type:Boolean,default:!0},scrollbarProps:Object,onScroll:Function,contentClass:String,contentStyle:{type:[String,Object],default:""},hasSider:Boolean,siderPlacement:{type:String,default:"left"}},Ee=Q("n-layout");function To(e){return E({name:"Layout",props:Object.assign(Object.assign({},U.props),No),setup(r){const o=F(null),a=F(null),{mergedClsPrefixRef:l,inlineThemeDisabled:i}=ie(r),m=U("Layout","-layout",Po,pe,r,l);function f(I,P){if(r.nativeScrollbar){const{value:k}=o;k&&(P===void 0?k.scrollTo(I):k.scrollTo(I,P))}else{const{value:k}=a;k&&k.scrollTo(I,P)}}Y(Ee,r);let c=0,p=0;const R=I=>{var P;const k=I.target;c=k.scrollLeft,p=k.scrollTop,(P=r.onScroll)===null||P===void 0||P.call(r,I)};_e(()=>{if(r.nativeScrollbar){const I=o.value;I&&(I.scrollTop=p,I.scrollLeft=c)}});const w={display:"flex",flexWrap:"nowrap",width:"100%",flexDirection:"row"},h={scrollTo:f},H=b(()=>{const{common:{cubicBezierEaseInOut:I},self:P}=m.value;return{"--n-bezier":I,"--n-color":r.embedded?P.colorEmbedded:P.color,"--n-text-color":P.textColor}}),A=i?le("layout",b(()=>r.embedded?"e":""),H,r):void 0;return Object.assign({mergedClsPrefix:l,scrollableElRef:o,scrollbarInstRef:a,hasSiderStyle:w,mergedTheme:m,handleNativeElScroll:R,cssVars:i?void 0:H,themeClass:A==null?void 0:A.themeClass,onRender:A==null?void 0:A.onRender},h)},render(){var r;const{mergedClsPrefix:o,hasSider:a}=this;(r=this.onRender)===null||r===void 0||r.call(this);const l=a?this.hasSiderStyle:void 0,i=[this.themeClass,e,`${o}-layout`,`${o}-layout--${this.position}-positioned`];return d("div",{class:i,style:this.cssVars},this.nativeScrollbar?d("div",{ref:"scrollableElRef",class:[`${o}-layout-scroll-container`,this.contentClass],style:[this.contentStyle,l],onScroll:this.handleNativeElScroll},this.$slots):d(ke,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,contentClass:this.contentClass,contentStyle:[this.contentStyle,l]}),this.$slots))}})}const ue=To(!1),Ao=u("layout-header",`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 box-sizing: border-box;
 width: 100%;
 background-color: var(--n-color);
 color: var(--n-text-color);
`,[S("absolute-positioned",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 `),S("bordered",`
 border-bottom: solid 1px var(--n-border-color);
 `)]),ko={position:be,inverted:Boolean,bordered:{type:Boolean,default:!1}},_o=E({name:"LayoutHeader",props:Object.assign(Object.assign({},U.props),ko),setup(e){const{mergedClsPrefixRef:r,inlineThemeDisabled:o}=ie(e),a=U("Layout","-layout-header",Ao,pe,e,r),l=b(()=>{const{common:{cubicBezierEaseInOut:m},self:f}=a.value,c={"--n-bezier":m};return e.inverted?(c["--n-color"]=f.headerColorInverted,c["--n-text-color"]=f.textColorInverted,c["--n-border-color"]=f.headerBorderColorInverted):(c["--n-color"]=f.headerColor,c["--n-text-color"]=f.textColor,c["--n-border-color"]=f.headerBorderColor),c}),i=o?le("layout-header",b(()=>e.inverted?"a":"b"),l,e):void 0;return{mergedClsPrefix:r,cssVars:o?void 0:l,themeClass:i==null?void 0:i.themeClass,onRender:i==null?void 0:i.onRender}},render(){var e;const{mergedClsPrefix:r}=this;return(e=this.onRender)===null||e===void 0||e.call(this),d("div",{class:[`${r}-layout-header`,this.themeClass,this.position&&`${r}-layout-header--${this.position}-positioned`,this.bordered&&`${r}-layout-header--bordered`],style:this.cssVars},this.$slots)}}),Ho=u("layout-sider",`
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
`,[S("bordered",[s("border",`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 width: 1px;
 background-color: var(--n-border-color);
 transition: background-color .3s var(--n-bezier);
 `)]),s("left-placement",[S("bordered",[s("border",`
 right: 0;
 `)])]),S("right-placement",`
 justify-content: flex-start;
 `,[S("bordered",[s("border",`
 left: 0;
 `)]),S("collapsed",[u("layout-toggle-button",[u("base-icon",`
 transform: rotate(180deg);
 `)]),u("layout-toggle-bar",[z("&:hover",[s("top",{transform:"rotate(-12deg) scale(1.15) translateY(-2px)"}),s("bottom",{transform:"rotate(12deg) scale(1.15) translateY(2px)"})])])]),u("layout-toggle-button",`
 left: 0;
 transform: translateX(-50%) translateY(-50%);
 `,[u("base-icon",`
 transform: rotate(0);
 `)]),u("layout-toggle-bar",`
 left: -28px;
 transform: rotate(180deg);
 `,[z("&:hover",[s("top",{transform:"rotate(12deg) scale(1.15) translateY(-2px)"}),s("bottom",{transform:"rotate(-12deg) scale(1.15) translateY(2px)"})])])]),S("collapsed",[u("layout-toggle-bar",[z("&:hover",[s("top",{transform:"rotate(-12deg) scale(1.15) translateY(-2px)"}),s("bottom",{transform:"rotate(12deg) scale(1.15) translateY(2px)"})])]),u("layout-toggle-button",[u("base-icon",`
 transform: rotate(0);
 `)])]),u("layout-toggle-button",`
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
 `,[u("base-icon",`
 transition: transform .3s var(--n-bezier);
 transform: rotate(180deg);
 `)]),u("layout-toggle-bar",`
 cursor: pointer;
 height: 72px;
 width: 32px;
 position: absolute;
 top: calc(50% - 36px);
 right: -28px;
 `,[s("top, bottom",`
 position: absolute;
 width: 4px;
 border-radius: 2px;
 height: 38px;
 left: 14px;
 transition: 
 background-color .3s var(--n-bezier),
 transform .3s var(--n-bezier);
 `),s("bottom",`
 position: absolute;
 top: 34px;
 `),z("&:hover",[s("top",{transform:"rotate(12deg) scale(1.15) translateY(-2px)"}),s("bottom",{transform:"rotate(-12deg) scale(1.15) translateY(2px)"})]),s("top, bottom",{backgroundColor:"var(--n-toggle-bar-color)"}),z("&:hover",[s("top, bottom",{backgroundColor:"var(--n-toggle-bar-color-hover)"})])]),s("border",`
 position: absolute;
 top: 0;
 right: 0;
 bottom: 0;
 width: 1px;
 transition: background-color .3s var(--n-bezier);
 `),u("layout-sider-scroll-container",`
 flex-grow: 1;
 flex-shrink: 0;
 box-sizing: border-box;
 height: 100%;
 opacity: 0;
 transition: opacity .3s var(--n-bezier);
 max-width: 100%;
 `),S("show-content",[u("layout-sider-scroll-container",{opacity:1})]),S("absolute-positioned",`
 position: absolute;
 left: 0;
 top: 0;
 bottom: 0;
 `)]),Bo=E({props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return d("div",{onClick:this.onClick,class:`${e}-layout-toggle-bar`},d("div",{class:`${e}-layout-toggle-bar__top`}),d("div",{class:`${e}-layout-toggle-bar__bottom`}))}}),Eo=E({name:"LayoutToggleButton",props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return d("div",{class:`${e}-layout-toggle-button`,onClick:this.onClick},d(He,{clsPrefix:e},{default:()=>d(po,null)}))}}),Oo={position:be,bordered:Boolean,collapsedWidth:{type:Number,default:48},width:{type:[Number,String],default:272},contentClass:String,contentStyle:{type:[String,Object],default:""},collapseMode:{type:String,default:"transform"},collapsed:{type:Boolean,default:void 0},defaultCollapsed:Boolean,showCollapsedContent:{type:Boolean,default:!0},showTrigger:{type:[Boolean,String],default:!1},nativeScrollbar:{type:Boolean,default:!0},inverted:Boolean,scrollbarProps:Object,triggerClass:String,triggerStyle:[String,Object],collapsedTriggerClass:String,collapsedTriggerStyle:[String,Object],"onUpdate:collapsed":[Function,Array],onUpdateCollapsed:[Function,Array],onAfterEnter:Function,onAfterLeave:Function,onExpand:[Function,Array],onCollapse:[Function,Array],onScroll:Function},$o=E({name:"LayoutSider",props:Object.assign(Object.assign({},U.props),Oo),setup(e){const r=D(Ee),o=F(null),a=F(null),l=F(e.defaultCollapsed),i=me(ne(e,"collapsed"),l),m=b(()=>de(i.value?e.collapsedWidth:e.width)),f=b(()=>e.collapseMode!=="transform"?{}:{minWidth:de(e.width)}),c=b(()=>r?r.siderPlacement:"left");function p(T,C){if(e.nativeScrollbar){const{value:y}=o;y&&(C===void 0?y.scrollTo(T):y.scrollTo(T,C))}else{const{value:y}=a;y&&y.scrollTo(T,C)}}function R(){const{"onUpdate:collapsed":T,onUpdateCollapsed:C,onExpand:y,onCollapse:j}=e,{value:M}=i;C&&$(C,!M),T&&$(T,!M),l.value=!M,M?y&&$(y):j&&$(j)}let w=0,h=0;const H=T=>{var C;const y=T.target;w=y.scrollLeft,h=y.scrollTop,(C=e.onScroll)===null||C===void 0||C.call(e,T)};_e(()=>{if(e.nativeScrollbar){const T=o.value;T&&(T.scrollTop=h,T.scrollLeft=w)}}),Y(Be,{collapsedRef:i,collapseModeRef:ne(e,"collapseMode")});const{mergedClsPrefixRef:A,inlineThemeDisabled:I}=ie(e),P=U("Layout","-layout-sider",Ho,pe,e,A);function k(T){var C,y;T.propertyName==="max-width"&&(i.value?(C=e.onAfterLeave)===null||C===void 0||C.call(e):(y=e.onAfterEnter)===null||y===void 0||y.call(e))}const W={scrollTo:p},L=b(()=>{const{common:{cubicBezierEaseInOut:T},self:C}=P.value,{siderToggleButtonColor:y,siderToggleButtonBorder:j,siderToggleBarColor:M,siderToggleBarColorHover:ae}=C,B={"--n-bezier":T,"--n-toggle-button-color":y,"--n-toggle-button-border":j,"--n-toggle-bar-color":M,"--n-toggle-bar-color-hover":ae};return e.inverted?(B["--n-color"]=C.siderColorInverted,B["--n-text-color"]=C.textColorInverted,B["--n-border-color"]=C.siderBorderColorInverted,B["--n-toggle-button-icon-color"]=C.siderToggleButtonIconColorInverted,B.__invertScrollbar=C.__invertScrollbar):(B["--n-color"]=C.siderColor,B["--n-text-color"]=C.textColor,B["--n-border-color"]=C.siderBorderColor,B["--n-toggle-button-icon-color"]=C.siderToggleButtonIconColor),B}),O=I?le("layout-sider",b(()=>e.inverted?"a":"b"),L,e):void 0;return Object.assign({scrollableElRef:o,scrollbarInstRef:a,mergedClsPrefix:A,mergedTheme:P,styleMaxWidth:m,mergedCollapsed:i,scrollContainerStyle:f,siderPlacement:c,handleNativeElScroll:H,handleTransitionend:k,handleTriggerClick:R,inlineThemeDisabled:I,cssVars:L,themeClass:O==null?void 0:O.themeClass,onRender:O==null?void 0:O.onRender},W)},render(){var e;const{mergedClsPrefix:r,mergedCollapsed:o,showTrigger:a}=this;return(e=this.onRender)===null||e===void 0||e.call(this),d("aside",{class:[`${r}-layout-sider`,this.themeClass,`${r}-layout-sider--${this.position}-positioned`,`${r}-layout-sider--${this.siderPlacement}-placement`,this.bordered&&`${r}-layout-sider--bordered`,o&&`${r}-layout-sider--collapsed`,(!o||this.showCollapsedContent)&&`${r}-layout-sider--show-content`],onTransitionend:this.handleTransitionend,style:[this.inlineThemeDisabled?void 0:this.cssVars,{maxWidth:this.styleMaxWidth,width:de(this.width)}]},this.nativeScrollbar?d("div",{class:[`${r}-layout-sider-scroll-container`,this.contentClass],onScroll:this.handleNativeElScroll,style:[this.scrollContainerStyle,{overflow:"auto"},this.contentStyle],ref:"scrollableElRef"},this.$slots):d(ke,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",style:this.scrollContainerStyle,contentStyle:this.contentStyle,contentClass:this.contentClass,theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,builtinThemeOverrides:this.inverted&&this.cssVars.__invertScrollbar==="true"?{colorHover:"rgba(255, 255, 255, .4)",color:"rgba(255, 255, 255, .3)"}:void 0}),this.$slots),a?a==="bar"?d(Bo,{clsPrefix:r,class:o?this.collapsedTriggerClass:this.triggerClass,style:o?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):d(Eo,{clsPrefix:r,class:o?this.collapsedTriggerClass:this.triggerClass,style:o?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):null,this.bordered?d("div",{class:`${r}-layout-sider__border`}):null)}}),J=Q("n-menu"),Oe=Q("n-submenu"),xe=Q("n-menu-item-group"),Te=[z("&::before","background-color: var(--n-item-color-hover);"),s("arrow",`
 color: var(--n-arrow-color-hover);
 `),s("icon",`
 color: var(--n-item-icon-color-hover);
 `),u("menu-item-content-header",`
 color: var(--n-item-text-color-hover);
 `,[z("a",`
 color: var(--n-item-text-color-hover);
 `),s("extra",`
 color: var(--n-item-text-color-hover);
 `)])],Ae=[s("icon",`
 color: var(--n-item-icon-color-hover-horizontal);
 `),u("menu-item-content-header",`
 color: var(--n-item-text-color-hover-horizontal);
 `,[z("a",`
 color: var(--n-item-text-color-hover-horizontal);
 `),s("extra",`
 color: var(--n-item-text-color-hover-horizontal);
 `)])],Fo=z([u("menu",`
 background-color: var(--n-color);
 color: var(--n-item-text-color);
 overflow: hidden;
 transition: background-color .3s var(--n-bezier);
 box-sizing: border-box;
 font-size: var(--n-font-size);
 padding-bottom: 6px;
 `,[S("horizontal",`
 max-width: 100%;
 width: 100%;
 display: flex;
 overflow: hidden;
 padding-bottom: 0;
 `,[u("submenu","margin: 0;"),u("menu-item","margin: 0;"),u("menu-item-content",`
 padding: 0 20px;
 border-bottom: 2px solid #0000;
 `,[z("&::before","display: none;"),S("selected","border-bottom: 2px solid var(--n-border-color-horizontal)")]),u("menu-item-content",[S("selected",[s("icon","color: var(--n-item-icon-color-active-horizontal);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-active-horizontal);
 `,[z("a","color: var(--n-item-text-color-active-horizontal);"),s("extra","color: var(--n-item-text-color-active-horizontal);")])]),S("child-active",`
 border-bottom: 2px solid var(--n-border-color-horizontal);
 `,[u("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-horizontal);
 `,[z("a",`
 color: var(--n-item-text-color-child-active-horizontal);
 `),s("extra",`
 color: var(--n-item-text-color-child-active-horizontal);
 `)]),s("icon",`
 color: var(--n-item-icon-color-child-active-horizontal);
 `)]),X("disabled",[X("selected, child-active",[z("&:focus-within",Ae)]),S("selected",[G(null,[s("icon","color: var(--n-item-icon-color-active-hover-horizontal);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover-horizontal);
 `,[z("a","color: var(--n-item-text-color-active-hover-horizontal);"),s("extra","color: var(--n-item-text-color-active-hover-horizontal);")])])]),S("child-active",[G(null,[s("icon","color: var(--n-item-icon-color-child-active-hover-horizontal);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover-horizontal);
 `,[z("a","color: var(--n-item-text-color-child-active-hover-horizontal);"),s("extra","color: var(--n-item-text-color-child-active-hover-horizontal);")])])]),G("border-bottom: 2px solid var(--n-border-color-horizontal);",Ae)]),u("menu-item-content-header",[z("a","color: var(--n-item-text-color-horizontal);")])])]),X("responsive",[u("menu-item-content-header",`
 overflow: hidden;
 text-overflow: ellipsis;
 `)]),S("collapsed",[u("menu-item-content",[S("selected",[z("&::before",`
 background-color: var(--n-item-color-active-collapsed) !important;
 `)]),u("menu-item-content-header","opacity: 0;"),s("arrow","opacity: 0;"),s("icon","color: var(--n-item-icon-color-collapsed);")])]),u("menu-item",`
 height: var(--n-item-height);
 margin-top: 6px;
 position: relative;
 `),u("menu-item-content",`
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
 `,[z("> *","z-index: 1;"),z("&::before",`
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
 `),S("disabled",`
 opacity: .45;
 cursor: not-allowed;
 `),S("collapsed",[s("arrow","transform: rotate(0);")]),S("selected",[z("&::before","background-color: var(--n-item-color-active);"),s("arrow","color: var(--n-arrow-color-active);"),s("icon","color: var(--n-item-icon-color-active);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-active);
 `,[z("a","color: var(--n-item-text-color-active);"),s("extra","color: var(--n-item-text-color-active);")])]),S("child-active",[u("menu-item-content-header",`
 color: var(--n-item-text-color-child-active);
 `,[z("a",`
 color: var(--n-item-text-color-child-active);
 `),s("extra",`
 color: var(--n-item-text-color-child-active);
 `)]),s("arrow",`
 color: var(--n-arrow-color-child-active);
 `),s("icon",`
 color: var(--n-item-icon-color-child-active);
 `)]),X("disabled",[X("selected, child-active",[z("&:focus-within",Te)]),S("selected",[G(null,[s("arrow","color: var(--n-arrow-color-active-hover);"),s("icon","color: var(--n-item-icon-color-active-hover);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover);
 `,[z("a","color: var(--n-item-text-color-active-hover);"),s("extra","color: var(--n-item-text-color-active-hover);")])])]),S("child-active",[G(null,[s("arrow","color: var(--n-arrow-color-child-active-hover);"),s("icon","color: var(--n-item-icon-color-child-active-hover);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover);
 `,[z("a","color: var(--n-item-text-color-child-active-hover);"),s("extra","color: var(--n-item-text-color-child-active-hover);")])])]),S("selected",[G(null,[z("&::before","background-color: var(--n-item-color-active-hover);")])]),G(null,Te)]),s("icon",`
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
 `),s("arrow",`
 grid-area: arrow;
 font-size: 16px;
 color: var(--n-arrow-color);
 transform: rotate(180deg);
 opacity: 1;
 transition:
 color .3s var(--n-bezier),
 transform 0.2s var(--n-bezier),
 opacity 0.2s var(--n-bezier);
 `),u("menu-item-content-header",`
 grid-area: content;
 transition:
 color .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 opacity: 1;
 white-space: nowrap;
 color: var(--n-item-text-color);
 `,[z("a",`
 outline: none;
 text-decoration: none;
 transition: color .3s var(--n-bezier);
 color: var(--n-item-text-color);
 `,[z("&::before",`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),s("extra",`
 font-size: .93em;
 color: var(--n-group-text-color);
 transition: color .3s var(--n-bezier);
 `)])]),u("submenu",`
 cursor: pointer;
 position: relative;
 margin-top: 6px;
 `,[u("menu-item-content",`
 height: var(--n-item-height);
 `),u("submenu-children",`
 overflow: hidden;
 padding: 0;
 `,[eo({duration:".2s"})])]),u("menu-item-group",[u("menu-item-group-title",`
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
 `)])]),u("menu-tooltip",[z("a",`
 color: inherit;
 text-decoration: none;
 `)]),u("menu-divider",`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 6px 18px;
 `)]);function G(e,r){return[S("hover",e,r),z("&:hover",e,r)]}const $e=E({name:"MenuOptionContent",props:{collapsed:Boolean,disabled:Boolean,title:[String,Function],icon:Function,extra:[String,Function],showArrow:Boolean,childActive:Boolean,hover:Boolean,paddingLeft:Number,selected:Boolean,maxIconSize:{type:Number,required:!0},activeIconSize:{type:Number,required:!0},iconMarginRight:{type:Number,required:!0},clsPrefix:{type:String,required:!0},onClick:Function,tmNode:{type:Object,required:!0},isEllipsisPlaceholder:Boolean},setup(e){const{props:r}=D(J);return{menuProps:r,style:b(()=>{const{paddingLeft:o}=e;return{paddingLeft:o&&`${o}px`}}),iconStyle:b(()=>{const{maxIconSize:o,activeIconSize:a,iconMarginRight:l}=e;return{width:`${o}px`,height:`${o}px`,fontSize:`${a}px`,marginRight:`${l}px`}})}},render(){const{clsPrefix:e,tmNode:r,menuProps:{renderIcon:o,renderLabel:a,renderExtra:l,expandIcon:i}}=this,m=o?o(r.rawNode):q(this.icon);return d("div",{onClick:f=>{var c;(c=this.onClick)===null||c===void 0||c.call(this,f)},role:"none",class:[`${e}-menu-item-content`,{[`${e}-menu-item-content--selected`]:this.selected,[`${e}-menu-item-content--collapsed`]:this.collapsed,[`${e}-menu-item-content--child-active`]:this.childActive,[`${e}-menu-item-content--disabled`]:this.disabled,[`${e}-menu-item-content--hover`]:this.hover}],style:this.style},m&&d("div",{class:`${e}-menu-item-content__icon`,style:this.iconStyle,role:"none"},[m]),d("div",{class:`${e}-menu-item-content-header`,role:"none"},this.isEllipsisPlaceholder?this.title:a?a(r.rawNode):q(this.title),this.extra||l?d("span",{class:`${e}-menu-item-content-header__extra`}," ",l?l(r.rawNode):q(this.extra)):null),this.showArrow?d(He,{ariaHidden:!0,class:`${e}-menu-item-content__arrow`,clsPrefix:e},{default:()=>i?i(r.rawNode):d(wo,null)}):null)}}),re=8;function Ce(e){const r=D(J),{props:o,mergedCollapsedRef:a}=r,l=D(Oe,null),i=D(xe,null),m=b(()=>o.mode==="horizontal"),f=b(()=>m.value?o.dropdownPlacement:"tmNodes"in e?"right-start":"right"),c=b(()=>{var h;return Math.max((h=o.collapsedIconSize)!==null&&h!==void 0?h:o.iconSize,o.iconSize)}),p=b(()=>{var h;return!m.value&&e.root&&a.value&&(h=o.collapsedIconSize)!==null&&h!==void 0?h:o.iconSize}),R=b(()=>{if(m.value)return;const{collapsedWidth:h,indent:H,rootIndent:A}=o,{root:I,isGroup:P}=e,k=A===void 0?H:A;return I?a.value?h/2-c.value/2:k:i&&typeof i.paddingLeftRef.value=="number"?H/2+i.paddingLeftRef.value:l&&typeof l.paddingLeftRef.value=="number"?(P?H/2:H)+l.paddingLeftRef.value:0}),w=b(()=>{const{collapsedWidth:h,indent:H,rootIndent:A}=o,{value:I}=c,{root:P}=e;return m.value||!P||!a.value?re:(A===void 0?H:A)+I+re-(h+I)/2});return{dropdownPlacement:f,activeIconSize:p,maxIconSize:c,paddingLeft:R,iconMarginRight:w,NMenu:r,NSubmenu:l,NMenuOptionGroup:i}}const ye={internalKey:{type:[String,Number],required:!0},root:Boolean,isGroup:Boolean,level:{type:Number,required:!0},title:[String,Function],extra:[String,Function]},Mo=E({name:"MenuDivider",setup(){const e=D(J),{mergedClsPrefixRef:r,isHorizontalRef:o}=e;return()=>o.value?null:d("div",{class:`${r.value}-menu-divider`})}}),Fe=Object.assign(Object.assign({},ye),{tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function}),Lo=ge(Fe),jo=E({name:"MenuOption",props:Fe,setup(e){const r=Ce(e),{NSubmenu:o,NMenu:a,NMenuOptionGroup:l}=r,{props:i,mergedClsPrefixRef:m,mergedCollapsedRef:f}=a,c=o?o.mergedDisabledRef:l?l.mergedDisabledRef:{value:!1},p=b(()=>c.value||e.disabled);function R(h){const{onClick:H}=e;H&&H(h)}function w(h){p.value||(a.doSelect(e.internalKey,e.tmNode.rawNode),R(h))}return{mergedClsPrefix:m,dropdownPlacement:r.dropdownPlacement,paddingLeft:r.paddingLeft,iconMarginRight:r.iconMarginRight,maxIconSize:r.maxIconSize,activeIconSize:r.activeIconSize,mergedTheme:a.mergedThemeRef,menuProps:i,dropdownEnabled:ve(()=>e.root&&f.value&&i.mode!=="horizontal"&&!p.value),selected:ve(()=>a.mergedValueRef.value===e.internalKey),mergedDisabled:p,handleClick:w}},render(){const{mergedClsPrefix:e,mergedTheme:r,tmNode:o,menuProps:{renderLabel:a,nodeProps:l}}=this,i=l==null?void 0:l(o.rawNode);return d("div",Object.assign({},i,{role:"menuitem",class:[`${e}-menu-item`,i==null?void 0:i.class]}),d(bo,{theme:r.peers.Tooltip,themeOverrides:r.peerOverrides.Tooltip,trigger:"hover",placement:this.dropdownPlacement,disabled:!this.dropdownEnabled||this.title===void 0,internalExtraClass:["menu-tooltip"]},{default:()=>a?a(o.rawNode):q(this.title),trigger:()=>d($e,{tmNode:o,clsPrefix:e,paddingLeft:this.paddingLeft,iconMarginRight:this.iconMarginRight,maxIconSize:this.maxIconSize,activeIconSize:this.activeIconSize,selected:this.selected,title:this.title,extra:this.extra,disabled:this.mergedDisabled,icon:this.icon,onClick:this.handleClick})}))}}),Me=Object.assign(Object.assign({},ye),{tmNode:{type:Object,required:!0},tmNodes:{type:Array,required:!0}}),Ko=ge(Me),Vo=E({name:"MenuOptionGroup",props:Me,setup(e){const r=Ce(e),{NSubmenu:o}=r,a=b(()=>o!=null&&o.mergedDisabledRef.value?!0:e.tmNode.disabled);Y(xe,{paddingLeftRef:r.paddingLeft,mergedDisabledRef:a});const{mergedClsPrefixRef:l,props:i}=D(J);return function(){const{value:m}=l,f=r.paddingLeft.value,{nodeProps:c}=i,p=c==null?void 0:c(e.tmNode.rawNode);return d("div",{class:`${m}-menu-item-group`,role:"group"},d("div",Object.assign({},p,{class:[`${m}-menu-item-group-title`,p==null?void 0:p.class],style:[(p==null?void 0:p.style)||"",f!==void 0?`padding-left: ${f}px;`:""]}),q(e.title),e.extra?d(oo,null," ",q(e.extra)):null),d("div",null,e.tmNodes.map(R=>ze(R,i))))}}});function he(e){return e.type==="divider"||e.type==="render"}function Do(e){return e.type==="divider"}function ze(e,r){const{rawNode:o}=e,{show:a}=o;if(a===!1)return null;if(he(o))return Do(o)?d(Mo,Object.assign({key:e.key},o.props)):null;const{labelField:l}=r,{key:i,level:m,isGroup:f}=e,c=Object.assign(Object.assign({},o),{title:o.title||o[l],extra:o.titleExtra||o.extra,key:i,internalKey:i,level:m,root:m===0,isGroup:f});return e.children?e.isGroup?d(Vo,ce(c,Ko,{tmNode:e,tmNodes:e.children,key:i})):d(fe,ce(c,Uo,{key:i,rawNodes:o[r.childrenField],tmNodes:e.children,tmNode:e})):d(jo,ce(c,Lo,{key:i,tmNode:e}))}const Le=Object.assign(Object.assign({},ye),{rawNodes:{type:Array,default:()=>[]},tmNodes:{type:Array,default:()=>[]},tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function,domId:String,virtualChildActive:{type:Boolean,default:void 0},isEllipsisPlaceholder:Boolean}),Uo=ge(Le),fe=E({name:"Submenu",props:Le,setup(e){const r=Ce(e),{NMenu:o,NSubmenu:a}=r,{props:l,mergedCollapsedRef:i,mergedThemeRef:m}=o,f=b(()=>{const{disabled:h}=e;return a!=null&&a.mergedDisabledRef.value||l.disabled?!0:h}),c=F(!1);Y(Oe,{paddingLeftRef:r.paddingLeft,mergedDisabledRef:f}),Y(xe,null);function p(){const{onClick:h}=e;h&&h()}function R(){f.value||(i.value||o.toggleExpand(e.internalKey),p())}function w(h){c.value=h}return{menuProps:l,mergedTheme:m,doSelect:o.doSelect,inverted:o.invertedRef,isHorizontal:o.isHorizontalRef,mergedClsPrefix:o.mergedClsPrefixRef,maxIconSize:r.maxIconSize,activeIconSize:r.activeIconSize,iconMarginRight:r.iconMarginRight,dropdownPlacement:r.dropdownPlacement,dropdownShow:c,paddingLeft:r.paddingLeft,mergedDisabled:f,mergedValue:o.mergedValueRef,childActive:ve(()=>{var h;return(h=e.virtualChildActive)!==null&&h!==void 0?h:o.activePathRef.value.includes(e.internalKey)}),collapsed:b(()=>l.mode==="horizontal"?!1:i.value?!0:!o.mergedExpandedKeysRef.value.includes(e.internalKey)),dropdownEnabled:b(()=>!f.value&&(l.mode==="horizontal"||i.value)),handlePopoverShowChange:w,handleClick:R}},render(){var e;const{mergedClsPrefix:r,menuProps:{renderIcon:o,renderLabel:a}}=this,l=()=>{const{isHorizontal:m,paddingLeft:f,collapsed:c,mergedDisabled:p,maxIconSize:R,activeIconSize:w,title:h,childActive:H,icon:A,handleClick:I,menuProps:{nodeProps:P},dropdownShow:k,iconMarginRight:W,tmNode:L,mergedClsPrefix:O,isEllipsisPlaceholder:T,extra:C}=this,y=P==null?void 0:P(L.rawNode);return d("div",Object.assign({},y,{class:[`${O}-menu-item`,y==null?void 0:y.class],role:"menuitem"}),d($e,{tmNode:L,paddingLeft:f,collapsed:c,disabled:p,iconMarginRight:W,maxIconSize:R,activeIconSize:w,title:h,extra:C,showArrow:!m,childActive:H,clsPrefix:O,icon:A,hover:k,onClick:I,isEllipsisPlaceholder:T}))},i=()=>d(to,null,{default:()=>{const{tmNodes:m,collapsed:f}=this;return f?null:d("div",{class:`${r}-submenu-children`,role:"menu"},m.map(c=>ze(c,this.menuProps)))}});return this.root?d(yo,Object.assign({size:"large",trigger:"hover"},(e=this.menuProps)===null||e===void 0?void 0:e.dropdownProps,{themeOverrides:this.mergedTheme.peerOverrides.Dropdown,theme:this.mergedTheme.peers.Dropdown,builtinThemeOverrides:{fontSizeLarge:"14px",optionIconSizeLarge:"18px"},value:this.mergedValue,disabled:!this.dropdownEnabled,placement:this.dropdownPlacement,keyField:this.menuProps.keyField,labelField:this.menuProps.labelField,childrenField:this.menuProps.childrenField,onUpdateShow:this.handlePopoverShowChange,options:this.rawNodes,onSelect:this.doSelect,inverted:this.inverted,renderIcon:o,renderLabel:a}),{default:()=>d("div",{class:`${r}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},l(),this.isHorizontal?null:i())}):d("div",{class:`${r}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},l(),i())}}),Go=Object.assign(Object.assign({},U.props),{options:{type:Array,default:()=>[]},collapsed:{type:Boolean,default:void 0},collapsedWidth:{type:Number,default:48},iconSize:{type:Number,default:20},collapsedIconSize:{type:Number,default:24},rootIndent:Number,indent:{type:Number,default:32},labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},disabledField:{type:String,default:"disabled"},defaultExpandAll:Boolean,defaultExpandedKeys:Array,expandedKeys:Array,value:[String,Number],defaultValue:{type:[String,Number],default:null},mode:{type:String,default:"vertical"},watchProps:{type:Array,default:void 0},disabled:Boolean,show:{type:Boolean,default:!0},inverted:Boolean,"onUpdate:expandedKeys":[Function,Array],onUpdateExpandedKeys:[Function,Array],onUpdateValue:[Function,Array],"onUpdate:value":[Function,Array],expandIcon:Function,renderIcon:Function,renderLabel:Function,renderExtra:Function,dropdownProps:Object,accordion:Boolean,nodeProps:Function,dropdownPlacement:{type:String,default:"bottom"},responsive:Boolean,items:Array,onOpenNamesChange:[Function,Array],onSelect:[Function,Array],onExpandedNamesChange:[Function,Array],expandedNames:Array,defaultExpandedNames:Array}),qo=E({name:"Menu",inheritAttrs:!1,props:Go,setup(e){const{mergedClsPrefixRef:r,inlineThemeDisabled:o}=ie(e),a=U("Menu","-menu",Fo,lo,e,r),l=D(Be,null),i=b(()=>{var v;const{collapsed:x}=e;if(x!==void 0)return x;if(l){const{collapseModeRef:t,collapsedRef:g}=l;if(t.value==="width")return(v=g.value)!==null&&v!==void 0?v:!1}return!1}),m=b(()=>{const{keyField:v,childrenField:x,disabledField:t}=e;return se(e.items||e.options,{getIgnored(g){return he(g)},getChildren(g){return g[x]},getDisabled(g){return g[t]},getKey(g){var N;return(N=g[v])!==null&&N!==void 0?N:g.name}})}),f=b(()=>new Set(m.value.treeNodes.map(v=>v.key))),{watchProps:c}=e,p=F(null);c!=null&&c.includes("defaultValue")?we(()=>{p.value=e.defaultValue}):p.value=e.defaultValue;const R=ne(e,"value"),w=me(R,p),h=F([]),H=()=>{h.value=e.defaultExpandAll?m.value.getNonLeafKeys():e.defaultExpandedNames||e.defaultExpandedKeys||m.value.getPath(w.value,{includeSelf:!1}).keyPath};c!=null&&c.includes("defaultExpandedKeys")?we(H):H();const A=ho(e,["expandedNames","expandedKeys"]),I=me(A,h),P=b(()=>m.value.treeNodes),k=b(()=>m.value.getPath(w.value).keyPath);Y(J,{props:e,mergedCollapsedRef:i,mergedThemeRef:a,mergedValueRef:w,mergedExpandedKeysRef:I,activePathRef:k,mergedClsPrefixRef:r,isHorizontalRef:b(()=>e.mode==="horizontal"),invertedRef:ne(e,"inverted"),doSelect:W,toggleExpand:O});function W(v,x){const{"onUpdate:value":t,onUpdateValue:g,onSelect:N}=e;g&&$(g,v,x),t&&$(t,v,x),N&&$(N,v,x),p.value=v}function L(v){const{"onUpdate:expandedKeys":x,onUpdateExpandedKeys:t,onExpandedNamesChange:g,onOpenNamesChange:N}=e;x&&$(x,v),t&&$(t,v),g&&$(g,v),N&&$(N,v),h.value=v}function O(v){const x=Array.from(I.value),t=x.findIndex(g=>g===v);if(~t)x.splice(t,1);else{if(e.accordion&&f.value.has(v)){const g=x.findIndex(N=>f.value.has(N));g>-1&&x.splice(g,1)}x.push(v)}L(x)}const T=v=>{const x=m.value.getPath(v??w.value,{includeSelf:!1}).keyPath;if(!x.length)return;const t=Array.from(I.value),g=new Set([...t,...x]);e.accordion&&f.value.forEach(N=>{g.has(N)&&!x.includes(N)&&g.delete(N)}),L(Array.from(g))},C=b(()=>{const{inverted:v}=e,{common:{cubicBezierEaseInOut:x},self:t}=a.value,{borderRadius:g,borderColorHorizontal:N,fontSize:Ye,itemHeight:We,dividerColor:Xe}=t,n={"--n-divider-color":Xe,"--n-bezier":x,"--n-font-size":Ye,"--n-border-color-horizontal":N,"--n-border-radius":g,"--n-item-height":We};return v?(n["--n-group-text-color"]=t.groupTextColorInverted,n["--n-color"]=t.colorInverted,n["--n-item-text-color"]=t.itemTextColorInverted,n["--n-item-text-color-hover"]=t.itemTextColorHoverInverted,n["--n-item-text-color-active"]=t.itemTextColorActiveInverted,n["--n-item-text-color-child-active"]=t.itemTextColorChildActiveInverted,n["--n-item-text-color-child-active-hover"]=t.itemTextColorChildActiveInverted,n["--n-item-text-color-active-hover"]=t.itemTextColorActiveHoverInverted,n["--n-item-icon-color"]=t.itemIconColorInverted,n["--n-item-icon-color-hover"]=t.itemIconColorHoverInverted,n["--n-item-icon-color-active"]=t.itemIconColorActiveInverted,n["--n-item-icon-color-active-hover"]=t.itemIconColorActiveHoverInverted,n["--n-item-icon-color-child-active"]=t.itemIconColorChildActiveInverted,n["--n-item-icon-color-child-active-hover"]=t.itemIconColorChildActiveHoverInverted,n["--n-item-icon-color-collapsed"]=t.itemIconColorCollapsedInverted,n["--n-item-text-color-horizontal"]=t.itemTextColorHorizontalInverted,n["--n-item-text-color-hover-horizontal"]=t.itemTextColorHoverHorizontalInverted,n["--n-item-text-color-active-horizontal"]=t.itemTextColorActiveHorizontalInverted,n["--n-item-text-color-child-active-horizontal"]=t.itemTextColorChildActiveHorizontalInverted,n["--n-item-text-color-child-active-hover-horizontal"]=t.itemTextColorChildActiveHoverHorizontalInverted,n["--n-item-text-color-active-hover-horizontal"]=t.itemTextColorActiveHoverHorizontalInverted,n["--n-item-icon-color-horizontal"]=t.itemIconColorHorizontalInverted,n["--n-item-icon-color-hover-horizontal"]=t.itemIconColorHoverHorizontalInverted,n["--n-item-icon-color-active-horizontal"]=t.itemIconColorActiveHorizontalInverted,n["--n-item-icon-color-active-hover-horizontal"]=t.itemIconColorActiveHoverHorizontalInverted,n["--n-item-icon-color-child-active-horizontal"]=t.itemIconColorChildActiveHorizontalInverted,n["--n-item-icon-color-child-active-hover-horizontal"]=t.itemIconColorChildActiveHoverHorizontalInverted,n["--n-arrow-color"]=t.arrowColorInverted,n["--n-arrow-color-hover"]=t.arrowColorHoverInverted,n["--n-arrow-color-active"]=t.arrowColorActiveInverted,n["--n-arrow-color-active-hover"]=t.arrowColorActiveHoverInverted,n["--n-arrow-color-child-active"]=t.arrowColorChildActiveInverted,n["--n-arrow-color-child-active-hover"]=t.arrowColorChildActiveHoverInverted,n["--n-item-color-hover"]=t.itemColorHoverInverted,n["--n-item-color-active"]=t.itemColorActiveInverted,n["--n-item-color-active-hover"]=t.itemColorActiveHoverInverted,n["--n-item-color-active-collapsed"]=t.itemColorActiveCollapsedInverted):(n["--n-group-text-color"]=t.groupTextColor,n["--n-color"]=t.color,n["--n-item-text-color"]=t.itemTextColor,n["--n-item-text-color-hover"]=t.itemTextColorHover,n["--n-item-text-color-active"]=t.itemTextColorActive,n["--n-item-text-color-child-active"]=t.itemTextColorChildActive,n["--n-item-text-color-child-active-hover"]=t.itemTextColorChildActiveHover,n["--n-item-text-color-active-hover"]=t.itemTextColorActiveHover,n["--n-item-icon-color"]=t.itemIconColor,n["--n-item-icon-color-hover"]=t.itemIconColorHover,n["--n-item-icon-color-active"]=t.itemIconColorActive,n["--n-item-icon-color-active-hover"]=t.itemIconColorActiveHover,n["--n-item-icon-color-child-active"]=t.itemIconColorChildActive,n["--n-item-icon-color-child-active-hover"]=t.itemIconColorChildActiveHover,n["--n-item-icon-color-collapsed"]=t.itemIconColorCollapsed,n["--n-item-text-color-horizontal"]=t.itemTextColorHorizontal,n["--n-item-text-color-hover-horizontal"]=t.itemTextColorHoverHorizontal,n["--n-item-text-color-active-horizontal"]=t.itemTextColorActiveHorizontal,n["--n-item-text-color-child-active-horizontal"]=t.itemTextColorChildActiveHorizontal,n["--n-item-text-color-child-active-hover-horizontal"]=t.itemTextColorChildActiveHoverHorizontal,n["--n-item-text-color-active-hover-horizontal"]=t.itemTextColorActiveHoverHorizontal,n["--n-item-icon-color-horizontal"]=t.itemIconColorHorizontal,n["--n-item-icon-color-hover-horizontal"]=t.itemIconColorHoverHorizontal,n["--n-item-icon-color-active-horizontal"]=t.itemIconColorActiveHorizontal,n["--n-item-icon-color-active-hover-horizontal"]=t.itemIconColorActiveHoverHorizontal,n["--n-item-icon-color-child-active-horizontal"]=t.itemIconColorChildActiveHorizontal,n["--n-item-icon-color-child-active-hover-horizontal"]=t.itemIconColorChildActiveHoverHorizontal,n["--n-arrow-color"]=t.arrowColor,n["--n-arrow-color-hover"]=t.arrowColorHover,n["--n-arrow-color-active"]=t.arrowColorActive,n["--n-arrow-color-active-hover"]=t.arrowColorActiveHover,n["--n-arrow-color-child-active"]=t.arrowColorChildActive,n["--n-arrow-color-child-active-hover"]=t.arrowColorChildActiveHover,n["--n-item-color-hover"]=t.itemColorHover,n["--n-item-color-active"]=t.itemColorActive,n["--n-item-color-active-hover"]=t.itemColorActiveHover,n["--n-item-color-active-collapsed"]=t.itemColorActiveCollapsed),n}),y=o?le("menu",b(()=>e.inverted?"a":"b"),C,e):void 0,j=no(),M=F(null),ae=F(null);let B=!0;const Ie=()=>{var v;B?B=!1:(v=M.value)===null||v===void 0||v.sync({showAllItemsBeforeCalculate:!0})};function je(){return document.getElementById(j)}const ee=F(-1);function Ke(v){ee.value=e.options.length-v}function Ve(v){v||(ee.value=-1)}const De=b(()=>{const v=ee.value;return{children:v===-1?[]:e.options.slice(v)}}),Ue=b(()=>{const{childrenField:v,disabledField:x,keyField:t}=e;return se([De.value],{getIgnored(g){return he(g)},getChildren(g){return g[v]},getDisabled(g){return g[x]},getKey(g){var N;return(N=g[t])!==null&&N!==void 0?N:g.name}})}),Ge=b(()=>se([{}]).treeNodes[0]);function qe(){var v;if(ee.value===-1)return d(fe,{root:!0,level:0,key:"__ellpisisGroupPlaceholder__",internalKey:"__ellpisisGroupPlaceholder__",title:"···",tmNode:Ge.value,domId:j,isEllipsisPlaceholder:!0});const x=Ue.value.treeNodes[0],t=k.value,g=!!(!((v=x.children)===null||v===void 0)&&v.some(N=>t.includes(N.key)));return d(fe,{level:0,root:!0,key:"__ellpisisGroup__",internalKey:"__ellpisisGroup__",title:"···",virtualChildActive:g,tmNode:x,domId:j,rawNodes:x.rawNode.children||[],tmNodes:x.children||[],isEllipsisPlaceholder:!0})}return{mergedClsPrefix:r,controlledExpandedKeys:A,uncontrolledExpanededKeys:h,mergedExpandedKeys:I,uncontrolledValue:p,mergedValue:w,activePath:k,tmNodes:P,mergedTheme:a,mergedCollapsed:i,cssVars:o?void 0:C,themeClass:y==null?void 0:y.themeClass,overflowRef:M,counterRef:ae,updateCounter:()=>{},onResize:Ie,onUpdateOverflow:Ve,onUpdateCount:Ke,renderCounter:qe,getCounter:je,onRender:y==null?void 0:y.onRender,showOption:T,deriveResponsiveState:Ie}},render(){const{mergedClsPrefix:e,mode:r,themeClass:o,onRender:a}=this;a==null||a();const l=()=>this.tmNodes.map(c=>ze(c,this.$props)),m=r==="horizontal"&&this.responsive,f=()=>d("div",io(this.$attrs,{role:r==="horizontal"?"menubar":"menu",class:[`${e}-menu`,o,`${e}-menu--${r}`,m&&`${e}-menu--responsive`,this.mergedCollapsed&&`${e}-menu--collapsed`],style:this.cssVars}),m?d(xo,{ref:"overflowRef",onUpdateOverflow:this.onUpdateOverflow,getCounter:this.getCounter,onUpdateCount:this.onUpdateCount,updateCounter:this.updateCounter,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:l,counter:this.renderCounter}):l());return m?d(ro,{onResize:this.onResize},{default:f}):f()}}),Yo={class:"left"},Wo={class:"right"},Xo={class:"who"},Zo={class:"binding-error-body"},Qo=E({__name:"Shell",setup(e){const r=ao(),o=fo(),a=vo(),l=mo(),i=So(),m=[{label:"账号与直播间",key:"accounts"},{label:"房管",key:"moderation"},{label:"弹幕姬",key:"danmaku"},{label:"自定义弹幕姬",key:"custom"},{label:"统计",key:"stats"},{label:"日志",key:"logs"},{label:"管理",key:"admin"}],f=b(()=>o.list.map(R=>({label:`${R.accountName} @ ${R.roomId}${R.enabled?"":"（已停用）"}`,value:R.id})));co(()=>void o.refresh());function c(R){if(!l.hasRoute(R)){i.info("这个页面还没做");return}l.push({name:R})}function p(){r.logout().catch(()=>{}).finally(()=>l.push("/login"))}return(R,w)=>(te(),oe(_(ue),{"has-sider":"",position:"absolute"},{default:K(()=>[V(_($o),{bordered:"",width:180,"content-style":"padding-top: 12px"},{default:K(()=>[V(_(qo),{value:String(_(a).name),options:m,"onUpdate:value":c},null,8,["value"])]),_:1}),V(_(ue),null,{default:K(()=>[V(_(_o),{bordered:"",class:"header"},{default:K(()=>{var h;return[Z("div",Yo,[_(o).loading?(te(),oe(_(go),{key:0,size:"small"})):(te(),oe(_(Co),{key:1,value:_(o).currentId,options:f.value,placeholder:"没有可用的直播间",style:{width:"260px"},"onUpdate:value":_(o).select},null,8,["value","options","onUpdate:value"]))]),Z("div",Wo,[Z("span",Xo,Re((h=_(r).user)==null?void 0:h.username),1),V(_(Pe),{text:"",size:"small",onClick:p},{default:K(()=>[...w[1]||(w[1]=[Ne(" 退出 ",-1)])]),_:1})])]}),_:1}),_(o).loadError?(te(),oe(_(Io),{key:0,type:"error",title:"加载直播间列表失败",class:"binding-error-alert"},{default:K(()=>[Z("div",Zo,[Z("span",null,Re(_(o).loadError),1),V(_(Pe),{size:"small",onClick:w[0]||(w[0]=h=>_(o).refresh())},{default:K(()=>[...w[2]||(w[2]=[Ne("重试",-1)])]),_:1})])]),_:1})):so("",!0),V(_(ue),{"content-style":"padding: 16px"},{default:K(()=>[V(_(uo))]),_:1})]),_:1})]),_:1}))}}),at=zo(Qo,[["__scopeId","data-v-99099263"]]);export{at as default};
