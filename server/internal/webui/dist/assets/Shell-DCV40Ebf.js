import{e as O,h as d,P as Ye,Q as We,R as Xe,S as Se,c as Z,a as u,b as S,U as Ne,u as te,f as V,V as Te,x as re,n as b,r as F,p as Y,m as s,d as z,W as Ae,i as K,X as $,t as oe,Y as X,Z as Ze,_ as q,k as ge,$ as ve,a0 as Qe,a1 as ie,a2 as Je,a3 as eo,a4 as we,s as oo,a5 as to,a6 as ro,B as no,v as io,a7 as le,E as G,F as H,H as ae,D,M as ce,a8 as lo,K as ao,L as co,a9 as so,aa as uo,G as vo}from"./index-CTh2PwO9.js";import{N as ho,u as mo,a as fo,b as go}from"./bindings-DeYefWf3.js";import{N as po}from"./Dropdown-C3H2whZL.js";import{f as se,a as he,u as bo,_ as xo}from"./_plugin-vue_export-helper-Df3W42YB.js";import{C as Co,V as yo,c as de,N as zo}from"./Select-Dv_-Hp_Z.js";const Io=O({name:"ChevronDownFilled",render(){return d("svg",{viewBox:"0 0 16 16",fill:"none",xmlns:"http://www.w3.org/2000/svg"},d("path",{d:"M3.20041 5.73966C3.48226 5.43613 3.95681 5.41856 4.26034 5.70041L8 9.22652L11.7397 5.70041C12.0432 5.41856 12.5177 5.43613 12.7996 5.73966C13.0815 6.0432 13.0639 6.51775 12.7603 6.7996L8.51034 10.7996C8.22258 11.0668 7.77743 11.0668 7.48967 10.7996L3.23966 6.7996C2.93613 6.51775 2.91856 6.0432 3.20041 5.73966Z",fill:"currentColor"}))}});function So(e){const{baseColor:t,textColor2:r,bodyColor:a,cardColor:l,dividerColor:i,actionColor:h,scrollbarColor:m,scrollbarColorHover:c,invertedColor:p}=e;return{textColor:r,textColorInverted:"#FFF",color:a,colorEmbedded:h,headerColor:l,headerColorInverted:p,footerColor:h,footerColorInverted:p,headerBorderColor:i,headerBorderColorInverted:p,footerBorderColor:i,footerBorderColorInverted:p,siderBorderColor:i,siderBorderColorInverted:p,siderColor:l,siderColorInverted:p,siderToggleButtonBorder:`1px solid ${i}`,siderToggleButtonColor:t,siderToggleButtonIconColor:r,siderToggleButtonIconColorInverted:r,siderToggleBarColor:Se(a,m),siderToggleBarColorHover:Se(a,c),__invertScrollbar:"true"}}const pe=Ye({name:"Layout",common:Xe,peers:{Scrollbar:We},self:So}),_e=Z("n-layout-sider"),be={type:String,default:"static"},wo=u("layout",`
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
 `)]),Ro={embedded:Boolean,position:be,nativeScrollbar:{type:Boolean,default:!0},scrollbarProps:Object,onScroll:Function,contentClass:String,contentStyle:{type:[String,Object],default:""},hasSider:Boolean,siderPlacement:{type:String,default:"left"}},ke=Z("n-layout");function Po(e){return O({name:"Layout",props:Object.assign(Object.assign({},V.props),Ro),setup(t){const r=F(null),a=F(null),{mergedClsPrefixRef:l,inlineThemeDisabled:i}=te(t),h=V("Layout","-layout",wo,pe,t,l);function m(I,R){if(t.nativeScrollbar){const{value:_}=r;_&&(R===void 0?_.scrollTo(I):_.scrollTo(I,R))}else{const{value:_}=a;_&&_.scrollTo(I,R)}}Y(ke,t);let c=0,p=0;const w=I=>{var R;const _=I.target;c=_.scrollLeft,p=_.scrollTop,(R=t.onScroll)===null||R===void 0||R.call(t,I)};Te(()=>{if(t.nativeScrollbar){const I=r.value;I&&(I.scrollTop=p,I.scrollLeft=c)}});const N={display:"flex",flexWrap:"nowrap",width:"100%",flexDirection:"row"},f={scrollTo:m},k=b(()=>{const{common:{cubicBezierEaseInOut:I},self:R}=h.value;return{"--n-bezier":I,"--n-color":t.embedded?R.colorEmbedded:R.color,"--n-text-color":R.textColor}}),A=i?re("layout",b(()=>t.embedded?"e":""),k,t):void 0;return Object.assign({mergedClsPrefix:l,scrollableElRef:r,scrollbarInstRef:a,hasSiderStyle:N,mergedTheme:h,handleNativeElScroll:w,cssVars:i?void 0:k,themeClass:A==null?void 0:A.themeClass,onRender:A==null?void 0:A.onRender},f)},render(){var t;const{mergedClsPrefix:r,hasSider:a}=this;(t=this.onRender)===null||t===void 0||t.call(this);const l=a?this.hasSiderStyle:void 0,i=[this.themeClass,e,`${r}-layout`,`${r}-layout--${this.position}-positioned`];return d("div",{class:i,style:this.cssVars},this.nativeScrollbar?d("div",{ref:"scrollableElRef",class:[`${r}-layout-scroll-container`,this.contentClass],style:[this.contentStyle,l],onScroll:this.handleNativeElScroll},this.$slots):d(Ne,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,contentClass:this.contentClass,contentStyle:[this.contentStyle,l]}),this.$slots))}})}const ue=Po(!1),No=u("layout-header",`
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
 `)]),To={position:be,inverted:Boolean,bordered:{type:Boolean,default:!1}},Ao=O({name:"LayoutHeader",props:Object.assign(Object.assign({},V.props),To),setup(e){const{mergedClsPrefixRef:t,inlineThemeDisabled:r}=te(e),a=V("Layout","-layout-header",No,pe,e,t),l=b(()=>{const{common:{cubicBezierEaseInOut:h},self:m}=a.value,c={"--n-bezier":h};return e.inverted?(c["--n-color"]=m.headerColorInverted,c["--n-text-color"]=m.textColorInverted,c["--n-border-color"]=m.headerBorderColorInverted):(c["--n-color"]=m.headerColor,c["--n-text-color"]=m.textColor,c["--n-border-color"]=m.headerBorderColor),c}),i=r?re("layout-header",b(()=>e.inverted?"a":"b"),l,e):void 0;return{mergedClsPrefix:t,cssVars:r?void 0:l,themeClass:i==null?void 0:i.themeClass,onRender:i==null?void 0:i.onRender}},render(){var e;const{mergedClsPrefix:t}=this;return(e=this.onRender)===null||e===void 0||e.call(this),d("div",{class:[`${t}-layout-header`,this.themeClass,this.position&&`${t}-layout-header--${this.position}-positioned`,this.bordered&&`${t}-layout-header--bordered`],style:this.cssVars},this.$slots)}}),_o=u("layout-sider",`
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
 `)]),ko=O({props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return d("div",{onClick:this.onClick,class:`${e}-layout-toggle-bar`},d("div",{class:`${e}-layout-toggle-bar__top`}),d("div",{class:`${e}-layout-toggle-bar__bottom`}))}}),Ho=O({name:"LayoutToggleButton",props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return d("div",{class:`${e}-layout-toggle-button`,onClick:this.onClick},d(Ae,{clsPrefix:e},{default:()=>d(Co,null)}))}}),Bo={position:be,bordered:Boolean,collapsedWidth:{type:Number,default:48},width:{type:[Number,String],default:272},contentClass:String,contentStyle:{type:[String,Object],default:""},collapseMode:{type:String,default:"transform"},collapsed:{type:Boolean,default:void 0},defaultCollapsed:Boolean,showCollapsedContent:{type:Boolean,default:!0},showTrigger:{type:[Boolean,String],default:!1},nativeScrollbar:{type:Boolean,default:!0},inverted:Boolean,scrollbarProps:Object,triggerClass:String,triggerStyle:[String,Object],collapsedTriggerClass:String,collapsedTriggerStyle:[String,Object],"onUpdate:collapsed":[Function,Array],onUpdateCollapsed:[Function,Array],onAfterEnter:Function,onAfterLeave:Function,onExpand:[Function,Array],onCollapse:[Function,Array],onScroll:Function},Oo=O({name:"LayoutSider",props:Object.assign(Object.assign({},V.props),Bo),setup(e){const t=K(ke),r=F(null),a=F(null),l=F(e.defaultCollapsed),i=he(oe(e,"collapsed"),l),h=b(()=>se(i.value?e.collapsedWidth:e.width)),m=b(()=>e.collapseMode!=="transform"?{}:{minWidth:se(e.width)}),c=b(()=>t?t.siderPlacement:"left");function p(T,C){if(e.nativeScrollbar){const{value:y}=r;y&&(C===void 0?y.scrollTo(T):y.scrollTo(T,C))}else{const{value:y}=a;y&&y.scrollTo(T,C)}}function w(){const{"onUpdate:collapsed":T,onUpdateCollapsed:C,onExpand:y,onCollapse:j}=e,{value:M}=i;C&&$(C,!M),T&&$(T,!M),l.value=!M,M?y&&$(y):j&&$(j)}let N=0,f=0;const k=T=>{var C;const y=T.target;N=y.scrollLeft,f=y.scrollTop,(C=e.onScroll)===null||C===void 0||C.call(e,T)};Te(()=>{if(e.nativeScrollbar){const T=r.value;T&&(T.scrollTop=f,T.scrollLeft=N)}}),Y(_e,{collapsedRef:i,collapseModeRef:oe(e,"collapseMode")});const{mergedClsPrefixRef:A,inlineThemeDisabled:I}=te(e),R=V("Layout","-layout-sider",_o,pe,e,A);function _(T){var C,y;T.propertyName==="max-width"&&(i.value?(C=e.onAfterLeave)===null||C===void 0||C.call(e):(y=e.onAfterEnter)===null||y===void 0||y.call(e))}const W={scrollTo:p},L=b(()=>{const{common:{cubicBezierEaseInOut:T},self:C}=R.value,{siderToggleButtonColor:y,siderToggleButtonBorder:j,siderToggleBarColor:M,siderToggleBarColorHover:ne}=C,B={"--n-bezier":T,"--n-toggle-button-color":y,"--n-toggle-button-border":j,"--n-toggle-bar-color":M,"--n-toggle-bar-color-hover":ne};return e.inverted?(B["--n-color"]=C.siderColorInverted,B["--n-text-color"]=C.textColorInverted,B["--n-border-color"]=C.siderBorderColorInverted,B["--n-toggle-button-icon-color"]=C.siderToggleButtonIconColorInverted,B.__invertScrollbar=C.__invertScrollbar):(B["--n-color"]=C.siderColor,B["--n-text-color"]=C.textColor,B["--n-border-color"]=C.siderBorderColor,B["--n-toggle-button-icon-color"]=C.siderToggleButtonIconColor),B}),E=I?re("layout-sider",b(()=>e.inverted?"a":"b"),L,e):void 0;return Object.assign({scrollableElRef:r,scrollbarInstRef:a,mergedClsPrefix:A,mergedTheme:R,styleMaxWidth:h,mergedCollapsed:i,scrollContainerStyle:m,siderPlacement:c,handleNativeElScroll:k,handleTransitionend:_,handleTriggerClick:w,inlineThemeDisabled:I,cssVars:L,themeClass:E==null?void 0:E.themeClass,onRender:E==null?void 0:E.onRender},W)},render(){var e;const{mergedClsPrefix:t,mergedCollapsed:r,showTrigger:a}=this;return(e=this.onRender)===null||e===void 0||e.call(this),d("aside",{class:[`${t}-layout-sider`,this.themeClass,`${t}-layout-sider--${this.position}-positioned`,`${t}-layout-sider--${this.siderPlacement}-placement`,this.bordered&&`${t}-layout-sider--bordered`,r&&`${t}-layout-sider--collapsed`,(!r||this.showCollapsedContent)&&`${t}-layout-sider--show-content`],onTransitionend:this.handleTransitionend,style:[this.inlineThemeDisabled?void 0:this.cssVars,{maxWidth:this.styleMaxWidth,width:se(this.width)}]},this.nativeScrollbar?d("div",{class:[`${t}-layout-sider-scroll-container`,this.contentClass],onScroll:this.handleNativeElScroll,style:[this.scrollContainerStyle,{overflow:"auto"},this.contentStyle],ref:"scrollableElRef"},this.$slots):d(Ne,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",style:this.scrollContainerStyle,contentStyle:this.contentStyle,contentClass:this.contentClass,theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,builtinThemeOverrides:this.inverted&&this.cssVars.__invertScrollbar==="true"?{colorHover:"rgba(255, 255, 255, .4)",color:"rgba(255, 255, 255, .3)"}:void 0}),this.$slots),a?a==="bar"?d(ko,{clsPrefix:t,class:r?this.collapsedTriggerClass:this.triggerClass,style:r?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):d(Ho,{clsPrefix:t,class:r?this.collapsedTriggerClass:this.triggerClass,style:r?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):null,this.bordered?d("div",{class:`${t}-layout-sider__border`}):null)}}),Q=Z("n-menu"),He=Z("n-submenu"),xe=Z("n-menu-item-group"),Re=[z("&::before","background-color: var(--n-item-color-hover);"),s("arrow",`
 color: var(--n-arrow-color-hover);
 `),s("icon",`
 color: var(--n-item-icon-color-hover);
 `),u("menu-item-content-header",`
 color: var(--n-item-text-color-hover);
 `,[z("a",`
 color: var(--n-item-text-color-hover);
 `),s("extra",`
 color: var(--n-item-text-color-hover);
 `)])],Pe=[s("icon",`
 color: var(--n-item-icon-color-hover-horizontal);
 `),u("menu-item-content-header",`
 color: var(--n-item-text-color-hover-horizontal);
 `,[z("a",`
 color: var(--n-item-text-color-hover-horizontal);
 `),s("extra",`
 color: var(--n-item-text-color-hover-horizontal);
 `)])],Eo=z([u("menu",`
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
 `)]),X("disabled",[X("selected, child-active",[z("&:focus-within",Pe)]),S("selected",[U(null,[s("icon","color: var(--n-item-icon-color-active-hover-horizontal);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover-horizontal);
 `,[z("a","color: var(--n-item-text-color-active-hover-horizontal);"),s("extra","color: var(--n-item-text-color-active-hover-horizontal);")])])]),S("child-active",[U(null,[s("icon","color: var(--n-item-icon-color-child-active-hover-horizontal);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover-horizontal);
 `,[z("a","color: var(--n-item-text-color-child-active-hover-horizontal);"),s("extra","color: var(--n-item-text-color-child-active-hover-horizontal);")])])]),U("border-bottom: 2px solid var(--n-border-color-horizontal);",Pe)]),u("menu-item-content-header",[z("a","color: var(--n-item-text-color-horizontal);")])])]),X("responsive",[u("menu-item-content-header",`
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
 `)]),X("disabled",[X("selected, child-active",[z("&:focus-within",Re)]),S("selected",[U(null,[s("arrow","color: var(--n-arrow-color-active-hover);"),s("icon","color: var(--n-item-icon-color-active-hover);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover);
 `,[z("a","color: var(--n-item-text-color-active-hover);"),s("extra","color: var(--n-item-text-color-active-hover);")])])]),S("child-active",[U(null,[s("arrow","color: var(--n-arrow-color-child-active-hover);"),s("icon","color: var(--n-item-icon-color-child-active-hover);"),u("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover);
 `,[z("a","color: var(--n-item-text-color-child-active-hover);"),s("extra","color: var(--n-item-text-color-child-active-hover);")])])]),S("selected",[U(null,[z("&::before","background-color: var(--n-item-color-active-hover);")])]),U(null,Re)]),s("icon",`
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
 `,[Ze({duration:".2s"})])]),u("menu-item-group",[u("menu-item-group-title",`
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
 `)]);function U(e,t){return[S("hover",e,t),z("&:hover",e,t)]}const Be=O({name:"MenuOptionContent",props:{collapsed:Boolean,disabled:Boolean,title:[String,Function],icon:Function,extra:[String,Function],showArrow:Boolean,childActive:Boolean,hover:Boolean,paddingLeft:Number,selected:Boolean,maxIconSize:{type:Number,required:!0},activeIconSize:{type:Number,required:!0},iconMarginRight:{type:Number,required:!0},clsPrefix:{type:String,required:!0},onClick:Function,tmNode:{type:Object,required:!0},isEllipsisPlaceholder:Boolean},setup(e){const{props:t}=K(Q);return{menuProps:t,style:b(()=>{const{paddingLeft:r}=e;return{paddingLeft:r&&`${r}px`}}),iconStyle:b(()=>{const{maxIconSize:r,activeIconSize:a,iconMarginRight:l}=e;return{width:`${r}px`,height:`${r}px`,fontSize:`${a}px`,marginRight:`${l}px`}})}},render(){const{clsPrefix:e,tmNode:t,menuProps:{renderIcon:r,renderLabel:a,renderExtra:l,expandIcon:i}}=this,h=r?r(t.rawNode):q(this.icon);return d("div",{onClick:m=>{var c;(c=this.onClick)===null||c===void 0||c.call(this,m)},role:"none",class:[`${e}-menu-item-content`,{[`${e}-menu-item-content--selected`]:this.selected,[`${e}-menu-item-content--collapsed`]:this.collapsed,[`${e}-menu-item-content--child-active`]:this.childActive,[`${e}-menu-item-content--disabled`]:this.disabled,[`${e}-menu-item-content--hover`]:this.hover}],style:this.style},h&&d("div",{class:`${e}-menu-item-content__icon`,style:this.iconStyle,role:"none"},[h]),d("div",{class:`${e}-menu-item-content-header`,role:"none"},this.isEllipsisPlaceholder?this.title:a?a(t.rawNode):q(this.title),this.extra||l?d("span",{class:`${e}-menu-item-content-header__extra`}," ",l?l(t.rawNode):q(this.extra)):null),this.showArrow?d(Ae,{ariaHidden:!0,class:`${e}-menu-item-content__arrow`,clsPrefix:e},{default:()=>i?i(t.rawNode):d(Io,null)}):null)}}),ee=8;function Ce(e){const t=K(Q),{props:r,mergedCollapsedRef:a}=t,l=K(He,null),i=K(xe,null),h=b(()=>r.mode==="horizontal"),m=b(()=>h.value?r.dropdownPlacement:"tmNodes"in e?"right-start":"right"),c=b(()=>{var f;return Math.max((f=r.collapsedIconSize)!==null&&f!==void 0?f:r.iconSize,r.iconSize)}),p=b(()=>{var f;return!h.value&&e.root&&a.value&&(f=r.collapsedIconSize)!==null&&f!==void 0?f:r.iconSize}),w=b(()=>{if(h.value)return;const{collapsedWidth:f,indent:k,rootIndent:A}=r,{root:I,isGroup:R}=e,_=A===void 0?k:A;return I?a.value?f/2-c.value/2:_:i&&typeof i.paddingLeftRef.value=="number"?k/2+i.paddingLeftRef.value:l&&typeof l.paddingLeftRef.value=="number"?(R?k/2:k)+l.paddingLeftRef.value:0}),N=b(()=>{const{collapsedWidth:f,indent:k,rootIndent:A}=r,{value:I}=c,{root:R}=e;return h.value||!R||!a.value?ee:(A===void 0?k:A)+I+ee-(f+I)/2});return{dropdownPlacement:m,activeIconSize:p,maxIconSize:c,paddingLeft:w,iconMarginRight:N,NMenu:t,NSubmenu:l,NMenuOptionGroup:i}}const ye={internalKey:{type:[String,Number],required:!0},root:Boolean,isGroup:Boolean,level:{type:Number,required:!0},title:[String,Function],extra:[String,Function]},$o=O({name:"MenuDivider",setup(){const e=K(Q),{mergedClsPrefixRef:t,isHorizontalRef:r}=e;return()=>r.value?null:d("div",{class:`${t.value}-menu-divider`})}}),Oe=Object.assign(Object.assign({},ye),{tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function}),Fo=ge(Oe),Mo=O({name:"MenuOption",props:Oe,setup(e){const t=Ce(e),{NSubmenu:r,NMenu:a,NMenuOptionGroup:l}=t,{props:i,mergedClsPrefixRef:h,mergedCollapsedRef:m}=a,c=r?r.mergedDisabledRef:l?l.mergedDisabledRef:{value:!1},p=b(()=>c.value||e.disabled);function w(f){const{onClick:k}=e;k&&k(f)}function N(f){p.value||(a.doSelect(e.internalKey,e.tmNode.rawNode),w(f))}return{mergedClsPrefix:h,dropdownPlacement:t.dropdownPlacement,paddingLeft:t.paddingLeft,iconMarginRight:t.iconMarginRight,maxIconSize:t.maxIconSize,activeIconSize:t.activeIconSize,mergedTheme:a.mergedThemeRef,menuProps:i,dropdownEnabled:ve(()=>e.root&&m.value&&i.mode!=="horizontal"&&!p.value),selected:ve(()=>a.mergedValueRef.value===e.internalKey),mergedDisabled:p,handleClick:N}},render(){const{mergedClsPrefix:e,mergedTheme:t,tmNode:r,menuProps:{renderLabel:a,nodeProps:l}}=this,i=l==null?void 0:l(r.rawNode);return d("div",Object.assign({},i,{role:"menuitem",class:[`${e}-menu-item`,i==null?void 0:i.class]}),d(ho,{theme:t.peers.Tooltip,themeOverrides:t.peerOverrides.Tooltip,trigger:"hover",placement:this.dropdownPlacement,disabled:!this.dropdownEnabled||this.title===void 0,internalExtraClass:["menu-tooltip"]},{default:()=>a?a(r.rawNode):q(this.title),trigger:()=>d(Be,{tmNode:r,clsPrefix:e,paddingLeft:this.paddingLeft,iconMarginRight:this.iconMarginRight,maxIconSize:this.maxIconSize,activeIconSize:this.activeIconSize,selected:this.selected,title:this.title,extra:this.extra,disabled:this.mergedDisabled,icon:this.icon,onClick:this.handleClick})}))}}),Ee=Object.assign(Object.assign({},ye),{tmNode:{type:Object,required:!0},tmNodes:{type:Array,required:!0}}),Lo=ge(Ee),jo=O({name:"MenuOptionGroup",props:Ee,setup(e){const t=Ce(e),{NSubmenu:r}=t,a=b(()=>r!=null&&r.mergedDisabledRef.value?!0:e.tmNode.disabled);Y(xe,{paddingLeftRef:t.paddingLeft,mergedDisabledRef:a});const{mergedClsPrefixRef:l,props:i}=K(Q);return function(){const{value:h}=l,m=t.paddingLeft.value,{nodeProps:c}=i,p=c==null?void 0:c(e.tmNode.rawNode);return d("div",{class:`${h}-menu-item-group`,role:"group"},d("div",Object.assign({},p,{class:[`${h}-menu-item-group-title`,p==null?void 0:p.class],style:[(p==null?void 0:p.style)||"",m!==void 0?`padding-left: ${m}px;`:""]}),q(e.title),e.extra?d(Qe,null," ",q(e.extra)):null),d("div",null,e.tmNodes.map(w=>ze(w,i))))}}});function me(e){return e.type==="divider"||e.type==="render"}function Ko(e){return e.type==="divider"}function ze(e,t){const{rawNode:r}=e,{show:a}=r;if(a===!1)return null;if(me(r))return Ko(r)?d($o,Object.assign({key:e.key},r.props)):null;const{labelField:l}=t,{key:i,level:h,isGroup:m}=e,c=Object.assign(Object.assign({},r),{title:r.title||r[l],extra:r.titleExtra||r.extra,key:i,internalKey:i,level:h,root:h===0,isGroup:m});return e.children?e.isGroup?d(jo,ie(c,Lo,{tmNode:e,tmNodes:e.children,key:i})):d(fe,ie(c,Vo,{key:i,rawNodes:r[t.childrenField],tmNodes:e.children,tmNode:e})):d(Mo,ie(c,Fo,{key:i,tmNode:e}))}const $e=Object.assign(Object.assign({},ye),{rawNodes:{type:Array,default:()=>[]},tmNodes:{type:Array,default:()=>[]},tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function,domId:String,virtualChildActive:{type:Boolean,default:void 0},isEllipsisPlaceholder:Boolean}),Vo=ge($e),fe=O({name:"Submenu",props:$e,setup(e){const t=Ce(e),{NMenu:r,NSubmenu:a}=t,{props:l,mergedCollapsedRef:i,mergedThemeRef:h}=r,m=b(()=>{const{disabled:f}=e;return a!=null&&a.mergedDisabledRef.value||l.disabled?!0:f}),c=F(!1);Y(He,{paddingLeftRef:t.paddingLeft,mergedDisabledRef:m}),Y(xe,null);function p(){const{onClick:f}=e;f&&f()}function w(){m.value||(i.value||r.toggleExpand(e.internalKey),p())}function N(f){c.value=f}return{menuProps:l,mergedTheme:h,doSelect:r.doSelect,inverted:r.invertedRef,isHorizontal:r.isHorizontalRef,mergedClsPrefix:r.mergedClsPrefixRef,maxIconSize:t.maxIconSize,activeIconSize:t.activeIconSize,iconMarginRight:t.iconMarginRight,dropdownPlacement:t.dropdownPlacement,dropdownShow:c,paddingLeft:t.paddingLeft,mergedDisabled:m,mergedValue:r.mergedValueRef,childActive:ve(()=>{var f;return(f=e.virtualChildActive)!==null&&f!==void 0?f:r.activePathRef.value.includes(e.internalKey)}),collapsed:b(()=>l.mode==="horizontal"?!1:i.value?!0:!r.mergedExpandedKeysRef.value.includes(e.internalKey)),dropdownEnabled:b(()=>!m.value&&(l.mode==="horizontal"||i.value)),handlePopoverShowChange:N,handleClick:w}},render(){var e;const{mergedClsPrefix:t,menuProps:{renderIcon:r,renderLabel:a}}=this,l=()=>{const{isHorizontal:h,paddingLeft:m,collapsed:c,mergedDisabled:p,maxIconSize:w,activeIconSize:N,title:f,childActive:k,icon:A,handleClick:I,menuProps:{nodeProps:R},dropdownShow:_,iconMarginRight:W,tmNode:L,mergedClsPrefix:E,isEllipsisPlaceholder:T,extra:C}=this,y=R==null?void 0:R(L.rawNode);return d("div",Object.assign({},y,{class:[`${E}-menu-item`,y==null?void 0:y.class],role:"menuitem"}),d(Be,{tmNode:L,paddingLeft:m,collapsed:c,disabled:p,iconMarginRight:W,maxIconSize:w,activeIconSize:N,title:f,extra:C,showArrow:!h,childActive:k,clsPrefix:E,icon:A,hover:_,onClick:I,isEllipsisPlaceholder:T}))},i=()=>d(Je,null,{default:()=>{const{tmNodes:h,collapsed:m}=this;return m?null:d("div",{class:`${t}-submenu-children`,role:"menu"},h.map(c=>ze(c,this.menuProps)))}});return this.root?d(po,Object.assign({size:"large",trigger:"hover"},(e=this.menuProps)===null||e===void 0?void 0:e.dropdownProps,{themeOverrides:this.mergedTheme.peerOverrides.Dropdown,theme:this.mergedTheme.peers.Dropdown,builtinThemeOverrides:{fontSizeLarge:"14px",optionIconSizeLarge:"18px"},value:this.mergedValue,disabled:!this.dropdownEnabled,placement:this.dropdownPlacement,keyField:this.menuProps.keyField,labelField:this.menuProps.labelField,childrenField:this.menuProps.childrenField,onUpdateShow:this.handlePopoverShowChange,options:this.rawNodes,onSelect:this.doSelect,inverted:this.inverted,renderIcon:r,renderLabel:a}),{default:()=>d("div",{class:`${t}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},l(),this.isHorizontal?null:i())}):d("div",{class:`${t}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},l(),i())}}),Do=Object.assign(Object.assign({},V.props),{options:{type:Array,default:()=>[]},collapsed:{type:Boolean,default:void 0},collapsedWidth:{type:Number,default:48},iconSize:{type:Number,default:20},collapsedIconSize:{type:Number,default:24},rootIndent:Number,indent:{type:Number,default:32},labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},disabledField:{type:String,default:"disabled"},defaultExpandAll:Boolean,defaultExpandedKeys:Array,expandedKeys:Array,value:[String,Number],defaultValue:{type:[String,Number],default:null},mode:{type:String,default:"vertical"},watchProps:{type:Array,default:void 0},disabled:Boolean,show:{type:Boolean,default:!0},inverted:Boolean,"onUpdate:expandedKeys":[Function,Array],onUpdateExpandedKeys:[Function,Array],onUpdateValue:[Function,Array],"onUpdate:value":[Function,Array],expandIcon:Function,renderIcon:Function,renderLabel:Function,renderExtra:Function,dropdownProps:Object,accordion:Boolean,nodeProps:Function,dropdownPlacement:{type:String,default:"bottom"},responsive:Boolean,items:Array,onOpenNamesChange:[Function,Array],onSelect:[Function,Array],onExpandedNamesChange:[Function,Array],expandedNames:Array,defaultExpandedNames:Array}),Uo=O({name:"Menu",inheritAttrs:!1,props:Do,setup(e){const{mergedClsPrefixRef:t,inlineThemeDisabled:r}=te(e),a=V("Menu","-menu",Eo,ro,e,t),l=K(_e,null),i=b(()=>{var v;const{collapsed:x}=e;if(x!==void 0)return x;if(l){const{collapseModeRef:o,collapsedRef:g}=l;if(o.value==="width")return(v=g.value)!==null&&v!==void 0?v:!1}return!1}),h=b(()=>{const{keyField:v,childrenField:x,disabledField:o}=e;return de(e.items||e.options,{getIgnored(g){return me(g)},getChildren(g){return g[x]},getDisabled(g){return g[o]},getKey(g){var P;return(P=g[v])!==null&&P!==void 0?P:g.name}})}),m=b(()=>new Set(h.value.treeNodes.map(v=>v.key))),{watchProps:c}=e,p=F(null);c!=null&&c.includes("defaultValue")?we(()=>{p.value=e.defaultValue}):p.value=e.defaultValue;const w=oe(e,"value"),N=he(w,p),f=F([]),k=()=>{f.value=e.defaultExpandAll?h.value.getNonLeafKeys():e.defaultExpandedNames||e.defaultExpandedKeys||h.value.getPath(N.value,{includeSelf:!1}).keyPath};c!=null&&c.includes("defaultExpandedKeys")?we(k):k();const A=mo(e,["expandedNames","expandedKeys"]),I=he(A,f),R=b(()=>h.value.treeNodes),_=b(()=>h.value.getPath(N.value).keyPath);Y(Q,{props:e,mergedCollapsedRef:i,mergedThemeRef:a,mergedValueRef:N,mergedExpandedKeysRef:I,activePathRef:_,mergedClsPrefixRef:t,isHorizontalRef:b(()=>e.mode==="horizontal"),invertedRef:oe(e,"inverted"),doSelect:W,toggleExpand:E});function W(v,x){const{"onUpdate:value":o,onUpdateValue:g,onSelect:P}=e;g&&$(g,v,x),o&&$(o,v,x),P&&$(P,v,x),p.value=v}function L(v){const{"onUpdate:expandedKeys":x,onUpdateExpandedKeys:o,onExpandedNamesChange:g,onOpenNamesChange:P}=e;x&&$(x,v),o&&$(o,v),g&&$(g,v),P&&$(P,v),f.value=v}function E(v){const x=Array.from(I.value),o=x.findIndex(g=>g===v);if(~o)x.splice(o,1);else{if(e.accordion&&m.value.has(v)){const g=x.findIndex(P=>m.value.has(P));g>-1&&x.splice(g,1)}x.push(v)}L(x)}const T=v=>{const x=h.value.getPath(v??N.value,{includeSelf:!1}).keyPath;if(!x.length)return;const o=Array.from(I.value),g=new Set([...o,...x]);e.accordion&&m.value.forEach(P=>{g.has(P)&&!x.includes(P)&&g.delete(P)}),L(Array.from(g))},C=b(()=>{const{inverted:v}=e,{common:{cubicBezierEaseInOut:x},self:o}=a.value,{borderRadius:g,borderColorHorizontal:P,fontSize:Ue,itemHeight:Ge,dividerColor:qe}=o,n={"--n-divider-color":qe,"--n-bezier":x,"--n-font-size":Ue,"--n-border-color-horizontal":P,"--n-border-radius":g,"--n-item-height":Ge};return v?(n["--n-group-text-color"]=o.groupTextColorInverted,n["--n-color"]=o.colorInverted,n["--n-item-text-color"]=o.itemTextColorInverted,n["--n-item-text-color-hover"]=o.itemTextColorHoverInverted,n["--n-item-text-color-active"]=o.itemTextColorActiveInverted,n["--n-item-text-color-child-active"]=o.itemTextColorChildActiveInverted,n["--n-item-text-color-child-active-hover"]=o.itemTextColorChildActiveInverted,n["--n-item-text-color-active-hover"]=o.itemTextColorActiveHoverInverted,n["--n-item-icon-color"]=o.itemIconColorInverted,n["--n-item-icon-color-hover"]=o.itemIconColorHoverInverted,n["--n-item-icon-color-active"]=o.itemIconColorActiveInverted,n["--n-item-icon-color-active-hover"]=o.itemIconColorActiveHoverInverted,n["--n-item-icon-color-child-active"]=o.itemIconColorChildActiveInverted,n["--n-item-icon-color-child-active-hover"]=o.itemIconColorChildActiveHoverInverted,n["--n-item-icon-color-collapsed"]=o.itemIconColorCollapsedInverted,n["--n-item-text-color-horizontal"]=o.itemTextColorHorizontalInverted,n["--n-item-text-color-hover-horizontal"]=o.itemTextColorHoverHorizontalInverted,n["--n-item-text-color-active-horizontal"]=o.itemTextColorActiveHorizontalInverted,n["--n-item-text-color-child-active-horizontal"]=o.itemTextColorChildActiveHorizontalInverted,n["--n-item-text-color-child-active-hover-horizontal"]=o.itemTextColorChildActiveHoverHorizontalInverted,n["--n-item-text-color-active-hover-horizontal"]=o.itemTextColorActiveHoverHorizontalInverted,n["--n-item-icon-color-horizontal"]=o.itemIconColorHorizontalInverted,n["--n-item-icon-color-hover-horizontal"]=o.itemIconColorHoverHorizontalInverted,n["--n-item-icon-color-active-horizontal"]=o.itemIconColorActiveHorizontalInverted,n["--n-item-icon-color-active-hover-horizontal"]=o.itemIconColorActiveHoverHorizontalInverted,n["--n-item-icon-color-child-active-horizontal"]=o.itemIconColorChildActiveHorizontalInverted,n["--n-item-icon-color-child-active-hover-horizontal"]=o.itemIconColorChildActiveHoverHorizontalInverted,n["--n-arrow-color"]=o.arrowColorInverted,n["--n-arrow-color-hover"]=o.arrowColorHoverInverted,n["--n-arrow-color-active"]=o.arrowColorActiveInverted,n["--n-arrow-color-active-hover"]=o.arrowColorActiveHoverInverted,n["--n-arrow-color-child-active"]=o.arrowColorChildActiveInverted,n["--n-arrow-color-child-active-hover"]=o.arrowColorChildActiveHoverInverted,n["--n-item-color-hover"]=o.itemColorHoverInverted,n["--n-item-color-active"]=o.itemColorActiveInverted,n["--n-item-color-active-hover"]=o.itemColorActiveHoverInverted,n["--n-item-color-active-collapsed"]=o.itemColorActiveCollapsedInverted):(n["--n-group-text-color"]=o.groupTextColor,n["--n-color"]=o.color,n["--n-item-text-color"]=o.itemTextColor,n["--n-item-text-color-hover"]=o.itemTextColorHover,n["--n-item-text-color-active"]=o.itemTextColorActive,n["--n-item-text-color-child-active"]=o.itemTextColorChildActive,n["--n-item-text-color-child-active-hover"]=o.itemTextColorChildActiveHover,n["--n-item-text-color-active-hover"]=o.itemTextColorActiveHover,n["--n-item-icon-color"]=o.itemIconColor,n["--n-item-icon-color-hover"]=o.itemIconColorHover,n["--n-item-icon-color-active"]=o.itemIconColorActive,n["--n-item-icon-color-active-hover"]=o.itemIconColorActiveHover,n["--n-item-icon-color-child-active"]=o.itemIconColorChildActive,n["--n-item-icon-color-child-active-hover"]=o.itemIconColorChildActiveHover,n["--n-item-icon-color-collapsed"]=o.itemIconColorCollapsed,n["--n-item-text-color-horizontal"]=o.itemTextColorHorizontal,n["--n-item-text-color-hover-horizontal"]=o.itemTextColorHoverHorizontal,n["--n-item-text-color-active-horizontal"]=o.itemTextColorActiveHorizontal,n["--n-item-text-color-child-active-horizontal"]=o.itemTextColorChildActiveHorizontal,n["--n-item-text-color-child-active-hover-horizontal"]=o.itemTextColorChildActiveHoverHorizontal,n["--n-item-text-color-active-hover-horizontal"]=o.itemTextColorActiveHoverHorizontal,n["--n-item-icon-color-horizontal"]=o.itemIconColorHorizontal,n["--n-item-icon-color-hover-horizontal"]=o.itemIconColorHoverHorizontal,n["--n-item-icon-color-active-horizontal"]=o.itemIconColorActiveHorizontal,n["--n-item-icon-color-active-hover-horizontal"]=o.itemIconColorActiveHoverHorizontal,n["--n-item-icon-color-child-active-horizontal"]=o.itemIconColorChildActiveHorizontal,n["--n-item-icon-color-child-active-hover-horizontal"]=o.itemIconColorChildActiveHoverHorizontal,n["--n-arrow-color"]=o.arrowColor,n["--n-arrow-color-hover"]=o.arrowColorHover,n["--n-arrow-color-active"]=o.arrowColorActive,n["--n-arrow-color-active-hover"]=o.arrowColorActiveHover,n["--n-arrow-color-child-active"]=o.arrowColorChildActive,n["--n-arrow-color-child-active-hover"]=o.arrowColorChildActiveHover,n["--n-item-color-hover"]=o.itemColorHover,n["--n-item-color-active"]=o.itemColorActive,n["--n-item-color-active-hover"]=o.itemColorActiveHover,n["--n-item-color-active-collapsed"]=o.itemColorActiveCollapsed),n}),y=r?re("menu",b(()=>e.inverted?"a":"b"),C,e):void 0,j=oo(),M=F(null),ne=F(null);let B=!0;const Ie=()=>{var v;B?B=!1:(v=M.value)===null||v===void 0||v.sync({showAllItemsBeforeCalculate:!0})};function Fe(){return document.getElementById(j)}const J=F(-1);function Me(v){J.value=e.options.length-v}function Le(v){v||(J.value=-1)}const je=b(()=>{const v=J.value;return{children:v===-1?[]:e.options.slice(v)}}),Ke=b(()=>{const{childrenField:v,disabledField:x,keyField:o}=e;return de([je.value],{getIgnored(g){return me(g)},getChildren(g){return g[v]},getDisabled(g){return g[x]},getKey(g){var P;return(P=g[o])!==null&&P!==void 0?P:g.name}})}),Ve=b(()=>de([{}]).treeNodes[0]);function De(){var v;if(J.value===-1)return d(fe,{root:!0,level:0,key:"__ellpisisGroupPlaceholder__",internalKey:"__ellpisisGroupPlaceholder__",title:"···",tmNode:Ve.value,domId:j,isEllipsisPlaceholder:!0});const x=Ke.value.treeNodes[0],o=_.value,g=!!(!((v=x.children)===null||v===void 0)&&v.some(P=>o.includes(P.key)));return d(fe,{level:0,root:!0,key:"__ellpisisGroup__",internalKey:"__ellpisisGroup__",title:"···",virtualChildActive:g,tmNode:x,domId:j,rawNodes:x.rawNode.children||[],tmNodes:x.children||[],isEllipsisPlaceholder:!0})}return{mergedClsPrefix:t,controlledExpandedKeys:A,uncontrolledExpanededKeys:f,mergedExpandedKeys:I,uncontrolledValue:p,mergedValue:N,activePath:_,tmNodes:R,mergedTheme:a,mergedCollapsed:i,cssVars:r?void 0:C,themeClass:y==null?void 0:y.themeClass,overflowRef:M,counterRef:ne,updateCounter:()=>{},onResize:Ie,onUpdateOverflow:Le,onUpdateCount:Me,renderCounter:De,getCounter:Fe,onRender:y==null?void 0:y.onRender,showOption:T,deriveResponsiveState:Ie}},render(){const{mergedClsPrefix:e,mode:t,themeClass:r,onRender:a}=this;a==null||a();const l=()=>this.tmNodes.map(c=>ze(c,this.$props)),h=t==="horizontal"&&this.responsive,m=()=>d("div",to(this.$attrs,{role:t==="horizontal"?"menubar":"menu",class:[`${e}-menu`,r,`${e}-menu--${t}`,h&&`${e}-menu--responsive`,this.mergedCollapsed&&`${e}-menu--collapsed`],style:this.cssVars}),h?d(yo,{ref:"overflowRef",onUpdateOverflow:this.onUpdateOverflow,getCounter:this.getCounter,onUpdateCount:this.onUpdateCount,updateCounter:this.updateCounter,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:l,counter:this.renderCounter}):l());return h?d(eo,{onResize:this.onResize},{default:m}):m()}}),Go={class:"left"},qo={class:"right"},Yo={class:"who"},Wo=O({__name:"Shell",setup(e){const t=no(),r=fo(),a=uo(),l=vo(),i=bo(),h=[{label:"账号与直播间",key:"accounts"},{label:"房管",key:"moderation"},{label:"弹幕姬",key:"danmaku"},{label:"自定义弹幕姬",key:"custom"},{label:"统计",key:"stats"},{label:"日志",key:"logs"},{label:"管理",key:"admin"}],m=b(()=>r.list.map(w=>({label:`${w.accountName} @ ${w.roomId}${w.enabled?"":"（已停用）"}`,value:w.id})));io(()=>void r.refresh());function c(w){if(!l.hasRoute(w)){i.info("这个页面还没做");return}l.push({name:w})}function p(){t.logout().catch(()=>{}).finally(()=>l.push("/login"))}return(w,N)=>(ae(),le(H(ue),{"has-sider":"",position:"absolute"},{default:G(()=>[D(H(Oo),{bordered:"",width:180,"content-style":"padding-top: 12px"},{default:G(()=>[D(H(Uo),{value:String(H(a).name),options:h,"onUpdate:value":c},null,8,["value"])]),_:1}),D(H(ue),null,{default:G(()=>[D(H(Ao),{bordered:"",class:"header"},{default:G(()=>{var f;return[ce("div",Go,[H(r).loading?(ae(),le(H(go),{key:0,size:"small"})):(ae(),le(H(zo),{key:1,value:H(r).currentId,options:m.value,placeholder:"没有可用的直播间",style:{width:"260px"},"onUpdate:value":H(r).select},null,8,["value","options","onUpdate:value"]))]),ce("div",qo,[ce("span",Yo,lo((f=H(t).user)==null?void 0:f.username),1),D(H(ao),{text:"",size:"small",onClick:p},{default:G(()=>[...N[0]||(N[0]=[co(" 退出 ",-1)])]),_:1})])]}),_:1}),D(H(ue),{"content-style":"padding: 16px"},{default:G(()=>[D(H(so))]),_:1})]),_:1})]),_:1}))}}),ot=xo(Wo,[["__scopeId","data-v-c0865bc2"]]);export{ot as default};
