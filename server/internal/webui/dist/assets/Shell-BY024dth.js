import{e as _,h as u,a as d,b as I,u as ue,f as q,x as ve,n as b,m as l,d as y,P as Ie,S as je,r as $,Q as Ve,i as j,R as O,t as ee,p as X,c as me,U as W,V as De,W as G,k as he,X as ae,Y as Ue,Z as te,_ as Ge,$ as qe,a0 as Ce,s as Ye,a1 as We,a2 as Xe,B as Ze,v as Qe,C as Je,E as U,D as B,G as eo,I as V,H as re,a3 as oo,L as to,M as ro,a4 as no,a5 as io,F as lo}from"./index-BsznBmGz.js";import{u as ao,a as co}from"./bindings-DYgdjGOb.js";import{p as we,l as Se,a as so,b as Re,N as ne}from"./Layout-ByzgoO-w.js";import{C as uo,N as vo,V as mo,c as ie}from"./Tooltip-Bzhmub-g.js";import{N as ho}from"./Dropdown-XzCU038w.js";import{u as ce,_ as fo}from"./_plugin-vue_export-helper-7h7LVAwr.js";import{f as le}from"./get-qhDypvqw.js";import{u as go}from"./use-message-Di2NNvCu.js";const po=_({name:"ChevronDownFilled",render(){return u("svg",{viewBox:"0 0 16 16",fill:"none",xmlns:"http://www.w3.org/2000/svg"},u("path",{d:"M3.20041 5.73966C3.48226 5.43613 3.95681 5.41856 4.26034 5.70041L8 9.22652L11.7397 5.70041C12.0432 5.41856 12.5177 5.43613 12.7996 5.73966C13.0815 6.0432 13.0639 6.51775 12.7603 6.7996L8.51034 10.7996C8.22258 11.0668 7.77743 11.0668 7.48967 10.7996L3.23966 6.7996C2.93613 6.51775 2.91856 6.0432 3.20041 5.73966Z",fill:"currentColor"}))}}),bo=d("layout-header",`
 transition:
 color .3s var(--n-bezier),
 background-color .3s var(--n-bezier),
 box-shadow .3s var(--n-bezier),
 border-color .3s var(--n-bezier);
 box-sizing: border-box;
 width: 100%;
 background-color: var(--n-color);
 color: var(--n-text-color);
`,[I("absolute-positioned",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 `),I("bordered",`
 border-bottom: solid 1px var(--n-border-color);
 `)]),xo={position:we,inverted:Boolean,bordered:{type:Boolean,default:!1}},Co=_({name:"LayoutHeader",props:Object.assign(Object.assign({},q.props),xo),setup(e){const{mergedClsPrefixRef:r,inlineThemeDisabled:t}=ue(e),v=q("Layout","-layout-header",bo,Se,e,r),a=b(()=>{const{common:{cubicBezierEaseInOut:m},self:f}=v.value,c={"--n-bezier":m};return e.inverted?(c["--n-color"]=f.headerColorInverted,c["--n-text-color"]=f.textColorInverted,c["--n-border-color"]=f.headerBorderColorInverted):(c["--n-color"]=f.headerColor,c["--n-text-color"]=f.textColor,c["--n-border-color"]=f.headerBorderColor),c}),i=t?ve("layout-header",b(()=>e.inverted?"a":"b"),a,e):void 0;return{mergedClsPrefix:r,cssVars:t?void 0:a,themeClass:i==null?void 0:i.themeClass,onRender:i==null?void 0:i.onRender}},render(){var e;const{mergedClsPrefix:r}=this;return(e=this.onRender)===null||e===void 0||e.call(this),u("div",{class:[`${r}-layout-header`,this.themeClass,this.position&&`${r}-layout-header--${this.position}-positioned`,this.bordered&&`${r}-layout-header--bordered`],style:this.cssVars},this.$slots)}}),yo=d("layout-sider",`
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
`,[I("bordered",[l("border",`
 content: "";
 position: absolute;
 top: 0;
 bottom: 0;
 width: 1px;
 background-color: var(--n-border-color);
 transition: background-color .3s var(--n-bezier);
 `)]),l("left-placement",[I("bordered",[l("border",`
 right: 0;
 `)])]),I("right-placement",`
 justify-content: flex-start;
 `,[I("bordered",[l("border",`
 left: 0;
 `)]),I("collapsed",[d("layout-toggle-button",[d("base-icon",`
 transform: rotate(180deg);
 `)]),d("layout-toggle-bar",[y("&:hover",[l("top",{transform:"rotate(-12deg) scale(1.15) translateY(-2px)"}),l("bottom",{transform:"rotate(12deg) scale(1.15) translateY(2px)"})])])]),d("layout-toggle-button",`
 left: 0;
 transform: translateX(-50%) translateY(-50%);
 `,[d("base-icon",`
 transform: rotate(0);
 `)]),d("layout-toggle-bar",`
 left: -28px;
 transform: rotate(180deg);
 `,[y("&:hover",[l("top",{transform:"rotate(12deg) scale(1.15) translateY(-2px)"}),l("bottom",{transform:"rotate(-12deg) scale(1.15) translateY(2px)"})])])]),I("collapsed",[d("layout-toggle-bar",[y("&:hover",[l("top",{transform:"rotate(-12deg) scale(1.15) translateY(-2px)"}),l("bottom",{transform:"rotate(12deg) scale(1.15) translateY(2px)"})])]),d("layout-toggle-button",[d("base-icon",`
 transform: rotate(0);
 `)])]),d("layout-toggle-button",`
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
 `,[d("base-icon",`
 transition: transform .3s var(--n-bezier);
 transform: rotate(180deg);
 `)]),d("layout-toggle-bar",`
 cursor: pointer;
 height: 72px;
 width: 32px;
 position: absolute;
 top: calc(50% - 36px);
 right: -28px;
 `,[l("top, bottom",`
 position: absolute;
 width: 4px;
 border-radius: 2px;
 height: 38px;
 left: 14px;
 transition: 
 background-color .3s var(--n-bezier),
 transform .3s var(--n-bezier);
 `),l("bottom",`
 position: absolute;
 top: 34px;
 `),y("&:hover",[l("top",{transform:"rotate(12deg) scale(1.15) translateY(-2px)"}),l("bottom",{transform:"rotate(-12deg) scale(1.15) translateY(2px)"})]),l("top, bottom",{backgroundColor:"var(--n-toggle-bar-color)"}),y("&:hover",[l("top, bottom",{backgroundColor:"var(--n-toggle-bar-color-hover)"})])]),l("border",`
 position: absolute;
 top: 0;
 right: 0;
 bottom: 0;
 width: 1px;
 transition: background-color .3s var(--n-bezier);
 `),d("layout-sider-scroll-container",`
 flex-grow: 1;
 flex-shrink: 0;
 box-sizing: border-box;
 height: 100%;
 opacity: 0;
 transition: opacity .3s var(--n-bezier);
 max-width: 100%;
 `),I("show-content",[d("layout-sider-scroll-container",{opacity:1})]),I("absolute-positioned",`
 position: absolute;
 left: 0;
 top: 0;
 bottom: 0;
 `)]),zo=_({props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return u("div",{onClick:this.onClick,class:`${e}-layout-toggle-bar`},u("div",{class:`${e}-layout-toggle-bar__top`}),u("div",{class:`${e}-layout-toggle-bar__bottom`}))}}),Io=_({name:"LayoutToggleButton",props:{clsPrefix:{type:String,required:!0},onClick:Function},render(){const{clsPrefix:e}=this;return u("div",{class:`${e}-layout-toggle-button`,onClick:this.onClick},u(Ie,{clsPrefix:e},{default:()=>u(uo,null)}))}}),wo={position:we,bordered:Boolean,collapsedWidth:{type:Number,default:48},width:{type:[Number,String],default:272},contentClass:String,contentStyle:{type:[String,Object],default:""},collapseMode:{type:String,default:"transform"},collapsed:{type:Boolean,default:void 0},defaultCollapsed:Boolean,showCollapsedContent:{type:Boolean,default:!0},showTrigger:{type:[Boolean,String],default:!1},nativeScrollbar:{type:Boolean,default:!0},inverted:Boolean,scrollbarProps:Object,triggerClass:String,triggerStyle:[String,Object],collapsedTriggerClass:String,collapsedTriggerStyle:[String,Object],"onUpdate:collapsed":[Function,Array],onUpdateCollapsed:[Function,Array],onAfterEnter:Function,onAfterLeave:Function,onExpand:[Function,Array],onCollapse:[Function,Array],onScroll:Function},So=_({name:"LayoutSider",props:Object.assign(Object.assign({},q.props),wo),setup(e){const r=j(so),t=$(null),v=$(null),a=$(e.defaultCollapsed),i=ce(ee(e,"collapsed"),a),m=b(()=>le(i.value?e.collapsedWidth:e.width)),f=b(()=>e.collapseMode!=="transform"?{}:{minWidth:le(e.width)}),c=b(()=>r?r.siderPlacement:"left");function z(S,x){if(e.nativeScrollbar){const{value:C}=t;C&&(x===void 0?C.scrollTo(S):C.scrollTo(S,x))}else{const{value:C}=v;C&&C.scrollTo(S,x)}}function P(){const{"onUpdate:collapsed":S,onUpdateCollapsed:x,onExpand:C,onCollapse:K}=e,{value:F}=i;x&&O(x,!F),S&&O(S,!F),a.value=!F,F?C&&O(C):K&&O(K)}let R=0,g=0;const N=S=>{var x;const C=S.target;R=C.scrollLeft,g=C.scrollTop,(x=e.onScroll)===null||x===void 0||x.call(e,S)};Ve(()=>{if(e.nativeScrollbar){const S=t.value;S&&(S.scrollTop=g,S.scrollLeft=R)}}),X(Re,{collapsedRef:i,collapseModeRef:ee(e,"collapseMode")});const{mergedClsPrefixRef:T,inlineThemeDisabled:A}=ue(e),k=q("Layout","-layout-sider",yo,Se,e,T);function M(S){var x,C;S.propertyName==="max-width"&&(i.value?(x=e.onAfterLeave)===null||x===void 0||x.call(e):(C=e.onAfterEnter)===null||C===void 0||C.call(e))}const Y={scrollTo:z},L=b(()=>{const{common:{cubicBezierEaseInOut:S},self:x}=k.value,{siderToggleButtonColor:C,siderToggleButtonBorder:K,siderToggleBarColor:F,siderToggleBarColorHover:oe}=x,H={"--n-bezier":S,"--n-toggle-button-color":C,"--n-toggle-button-border":K,"--n-toggle-bar-color":F,"--n-toggle-bar-color-hover":oe};return e.inverted?(H["--n-color"]=x.siderColorInverted,H["--n-text-color"]=x.textColorInverted,H["--n-border-color"]=x.siderBorderColorInverted,H["--n-toggle-button-icon-color"]=x.siderToggleButtonIconColorInverted,H.__invertScrollbar=x.__invertScrollbar):(H["--n-color"]=x.siderColor,H["--n-text-color"]=x.textColor,H["--n-border-color"]=x.siderBorderColor,H["--n-toggle-button-icon-color"]=x.siderToggleButtonIconColor),H}),E=A?ve("layout-sider",b(()=>e.inverted?"a":"b"),L,e):void 0;return Object.assign({scrollableElRef:t,scrollbarInstRef:v,mergedClsPrefix:T,mergedTheme:k,styleMaxWidth:m,mergedCollapsed:i,scrollContainerStyle:f,siderPlacement:c,handleNativeElScroll:N,handleTransitionend:M,handleTriggerClick:P,inlineThemeDisabled:A,cssVars:L,themeClass:E==null?void 0:E.themeClass,onRender:E==null?void 0:E.onRender},Y)},render(){var e;const{mergedClsPrefix:r,mergedCollapsed:t,showTrigger:v}=this;return(e=this.onRender)===null||e===void 0||e.call(this),u("aside",{class:[`${r}-layout-sider`,this.themeClass,`${r}-layout-sider--${this.position}-positioned`,`${r}-layout-sider--${this.siderPlacement}-placement`,this.bordered&&`${r}-layout-sider--bordered`,t&&`${r}-layout-sider--collapsed`,(!t||this.showCollapsedContent)&&`${r}-layout-sider--show-content`],onTransitionend:this.handleTransitionend,style:[this.inlineThemeDisabled?void 0:this.cssVars,{maxWidth:this.styleMaxWidth,width:le(this.width)}]},this.nativeScrollbar?u("div",{class:[`${r}-layout-sider-scroll-container`,this.contentClass],onScroll:this.handleNativeElScroll,style:[this.scrollContainerStyle,{overflow:"auto"},this.contentStyle],ref:"scrollableElRef"},this.$slots):u(je,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",style:this.scrollContainerStyle,contentStyle:this.contentStyle,contentClass:this.contentClass,theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,builtinThemeOverrides:this.inverted&&this.cssVars.__invertScrollbar==="true"?{colorHover:"rgba(255, 255, 255, .4)",color:"rgba(255, 255, 255, .3)"}:void 0}),this.$slots),v?v==="bar"?u(zo,{clsPrefix:r,class:t?this.collapsedTriggerClass:this.triggerClass,style:t?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):u(Io,{clsPrefix:r,class:t?this.collapsedTriggerClass:this.triggerClass,style:t?this.collapsedTriggerStyle:this.triggerStyle,onClick:this.handleTriggerClick}):null,this.bordered?u("div",{class:`${r}-layout-sider__border`}):null)}}),Z=me("n-menu"),Pe=me("n-submenu"),fe=me("n-menu-item-group"),ye=[y("&::before","background-color: var(--n-item-color-hover);"),l("arrow",`
 color: var(--n-arrow-color-hover);
 `),l("icon",`
 color: var(--n-item-icon-color-hover);
 `),d("menu-item-content-header",`
 color: var(--n-item-text-color-hover);
 `,[y("a",`
 color: var(--n-item-text-color-hover);
 `),l("extra",`
 color: var(--n-item-text-color-hover);
 `)])],ze=[l("icon",`
 color: var(--n-item-icon-color-hover-horizontal);
 `),d("menu-item-content-header",`
 color: var(--n-item-text-color-hover-horizontal);
 `,[y("a",`
 color: var(--n-item-text-color-hover-horizontal);
 `),l("extra",`
 color: var(--n-item-text-color-hover-horizontal);
 `)])],Ro=y([d("menu",`
 background-color: var(--n-color);
 color: var(--n-item-text-color);
 overflow: hidden;
 transition: background-color .3s var(--n-bezier);
 box-sizing: border-box;
 font-size: var(--n-font-size);
 padding-bottom: 6px;
 `,[I("horizontal",`
 max-width: 100%;
 width: 100%;
 display: flex;
 overflow: hidden;
 padding-bottom: 0;
 `,[d("submenu","margin: 0;"),d("menu-item","margin: 0;"),d("menu-item-content",`
 padding: 0 20px;
 border-bottom: 2px solid #0000;
 `,[y("&::before","display: none;"),I("selected","border-bottom: 2px solid var(--n-border-color-horizontal)")]),d("menu-item-content",[I("selected",[l("icon","color: var(--n-item-icon-color-active-horizontal);"),d("menu-item-content-header",`
 color: var(--n-item-text-color-active-horizontal);
 `,[y("a","color: var(--n-item-text-color-active-horizontal);"),l("extra","color: var(--n-item-text-color-active-horizontal);")])]),I("child-active",`
 border-bottom: 2px solid var(--n-border-color-horizontal);
 `,[d("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-horizontal);
 `,[y("a",`
 color: var(--n-item-text-color-child-active-horizontal);
 `),l("extra",`
 color: var(--n-item-text-color-child-active-horizontal);
 `)]),l("icon",`
 color: var(--n-item-icon-color-child-active-horizontal);
 `)]),W("disabled",[W("selected, child-active",[y("&:focus-within",ze)]),I("selected",[D(null,[l("icon","color: var(--n-item-icon-color-active-hover-horizontal);"),d("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover-horizontal);
 `,[y("a","color: var(--n-item-text-color-active-hover-horizontal);"),l("extra","color: var(--n-item-text-color-active-hover-horizontal);")])])]),I("child-active",[D(null,[l("icon","color: var(--n-item-icon-color-child-active-hover-horizontal);"),d("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover-horizontal);
 `,[y("a","color: var(--n-item-text-color-child-active-hover-horizontal);"),l("extra","color: var(--n-item-text-color-child-active-hover-horizontal);")])])]),D("border-bottom: 2px solid var(--n-border-color-horizontal);",ze)]),d("menu-item-content-header",[y("a","color: var(--n-item-text-color-horizontal);")])])]),W("responsive",[d("menu-item-content-header",`
 overflow: hidden;
 text-overflow: ellipsis;
 `)]),I("collapsed",[d("menu-item-content",[I("selected",[y("&::before",`
 background-color: var(--n-item-color-active-collapsed) !important;
 `)]),d("menu-item-content-header","opacity: 0;"),l("arrow","opacity: 0;"),l("icon","color: var(--n-item-icon-color-collapsed);")])]),d("menu-item",`
 height: var(--n-item-height);
 margin-top: 6px;
 position: relative;
 `),d("menu-item-content",`
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
 `,[y("> *","z-index: 1;"),y("&::before",`
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
 `),I("disabled",`
 opacity: .45;
 cursor: not-allowed;
 `),I("collapsed",[l("arrow","transform: rotate(0);")]),I("selected",[y("&::before","background-color: var(--n-item-color-active);"),l("arrow","color: var(--n-arrow-color-active);"),l("icon","color: var(--n-item-icon-color-active);"),d("menu-item-content-header",`
 color: var(--n-item-text-color-active);
 `,[y("a","color: var(--n-item-text-color-active);"),l("extra","color: var(--n-item-text-color-active);")])]),I("child-active",[d("menu-item-content-header",`
 color: var(--n-item-text-color-child-active);
 `,[y("a",`
 color: var(--n-item-text-color-child-active);
 `),l("extra",`
 color: var(--n-item-text-color-child-active);
 `)]),l("arrow",`
 color: var(--n-arrow-color-child-active);
 `),l("icon",`
 color: var(--n-item-icon-color-child-active);
 `)]),W("disabled",[W("selected, child-active",[y("&:focus-within",ye)]),I("selected",[D(null,[l("arrow","color: var(--n-arrow-color-active-hover);"),l("icon","color: var(--n-item-icon-color-active-hover);"),d("menu-item-content-header",`
 color: var(--n-item-text-color-active-hover);
 `,[y("a","color: var(--n-item-text-color-active-hover);"),l("extra","color: var(--n-item-text-color-active-hover);")])])]),I("child-active",[D(null,[l("arrow","color: var(--n-arrow-color-child-active-hover);"),l("icon","color: var(--n-item-icon-color-child-active-hover);"),d("menu-item-content-header",`
 color: var(--n-item-text-color-child-active-hover);
 `,[y("a","color: var(--n-item-text-color-child-active-hover);"),l("extra","color: var(--n-item-text-color-child-active-hover);")])])]),I("selected",[D(null,[y("&::before","background-color: var(--n-item-color-active-hover);")])]),D(null,ye)]),l("icon",`
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
 `),l("arrow",`
 grid-area: arrow;
 font-size: 16px;
 color: var(--n-arrow-color);
 transform: rotate(180deg);
 opacity: 1;
 transition:
 color .3s var(--n-bezier),
 transform 0.2s var(--n-bezier),
 opacity 0.2s var(--n-bezier);
 `),d("menu-item-content-header",`
 grid-area: content;
 transition:
 color .3s var(--n-bezier),
 opacity .3s var(--n-bezier);
 opacity: 1;
 white-space: nowrap;
 color: var(--n-item-text-color);
 `,[y("a",`
 outline: none;
 text-decoration: none;
 transition: color .3s var(--n-bezier);
 color: var(--n-item-text-color);
 `,[y("&::before",`
 content: "";
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),l("extra",`
 font-size: .93em;
 color: var(--n-group-text-color);
 transition: color .3s var(--n-bezier);
 `)])]),d("submenu",`
 cursor: pointer;
 position: relative;
 margin-top: 6px;
 `,[d("menu-item-content",`
 height: var(--n-item-height);
 `),d("submenu-children",`
 overflow: hidden;
 padding: 0;
 `,[De({duration:".2s"})])]),d("menu-item-group",[d("menu-item-group-title",`
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
 `)])]),d("menu-tooltip",[y("a",`
 color: inherit;
 text-decoration: none;
 `)]),d("menu-divider",`
 transition: background-color .3s var(--n-bezier);
 background-color: var(--n-divider-color);
 height: 1px;
 margin: 6px 18px;
 `)]);function D(e,r){return[I("hover",e,r),y("&:hover",e,r)]}const Ne=_({name:"MenuOptionContent",props:{collapsed:Boolean,disabled:Boolean,title:[String,Function],icon:Function,extra:[String,Function],showArrow:Boolean,childActive:Boolean,hover:Boolean,paddingLeft:Number,selected:Boolean,maxIconSize:{type:Number,required:!0},activeIconSize:{type:Number,required:!0},iconMarginRight:{type:Number,required:!0},clsPrefix:{type:String,required:!0},onClick:Function,tmNode:{type:Object,required:!0},isEllipsisPlaceholder:Boolean},setup(e){const{props:r}=j(Z);return{menuProps:r,style:b(()=>{const{paddingLeft:t}=e;return{paddingLeft:t&&`${t}px`}}),iconStyle:b(()=>{const{maxIconSize:t,activeIconSize:v,iconMarginRight:a}=e;return{width:`${t}px`,height:`${t}px`,fontSize:`${v}px`,marginRight:`${a}px`}})}},render(){const{clsPrefix:e,tmNode:r,menuProps:{renderIcon:t,renderLabel:v,renderExtra:a,expandIcon:i}}=this,m=t?t(r.rawNode):G(this.icon);return u("div",{onClick:f=>{var c;(c=this.onClick)===null||c===void 0||c.call(this,f)},role:"none",class:[`${e}-menu-item-content`,{[`${e}-menu-item-content--selected`]:this.selected,[`${e}-menu-item-content--collapsed`]:this.collapsed,[`${e}-menu-item-content--child-active`]:this.childActive,[`${e}-menu-item-content--disabled`]:this.disabled,[`${e}-menu-item-content--hover`]:this.hover}],style:this.style},m&&u("div",{class:`${e}-menu-item-content__icon`,style:this.iconStyle,role:"none"},[m]),u("div",{class:`${e}-menu-item-content-header`,role:"none"},this.isEllipsisPlaceholder?this.title:v?v(r.rawNode):G(this.title),this.extra||a?u("span",{class:`${e}-menu-item-content-header__extra`}," ",a?a(r.rawNode):G(this.extra)):null),this.showArrow?u(Ie,{ariaHidden:!0,class:`${e}-menu-item-content__arrow`,clsPrefix:e},{default:()=>i?i(r.rawNode):u(po,null)}):null)}}),J=8;function ge(e){const r=j(Z),{props:t,mergedCollapsedRef:v}=r,a=j(Pe,null),i=j(fe,null),m=b(()=>t.mode==="horizontal"),f=b(()=>m.value?t.dropdownPlacement:"tmNodes"in e?"right-start":"right"),c=b(()=>{var g;return Math.max((g=t.collapsedIconSize)!==null&&g!==void 0?g:t.iconSize,t.iconSize)}),z=b(()=>{var g;return!m.value&&e.root&&v.value&&(g=t.collapsedIconSize)!==null&&g!==void 0?g:t.iconSize}),P=b(()=>{if(m.value)return;const{collapsedWidth:g,indent:N,rootIndent:T}=t,{root:A,isGroup:k}=e,M=T===void 0?N:T;return A?v.value?g/2-c.value/2:M:i&&typeof i.paddingLeftRef.value=="number"?N/2+i.paddingLeftRef.value:a&&typeof a.paddingLeftRef.value=="number"?(k?N/2:N)+a.paddingLeftRef.value:0}),R=b(()=>{const{collapsedWidth:g,indent:N,rootIndent:T}=t,{value:A}=c,{root:k}=e;return m.value||!k||!v.value?J:(T===void 0?N:T)+A+J-(g+A)/2});return{dropdownPlacement:f,activeIconSize:z,maxIconSize:c,paddingLeft:P,iconMarginRight:R,NMenu:r,NSubmenu:a,NMenuOptionGroup:i}}const pe={internalKey:{type:[String,Number],required:!0},root:Boolean,isGroup:Boolean,level:{type:Number,required:!0},title:[String,Function],extra:[String,Function]},Po=_({name:"MenuDivider",setup(){const e=j(Z),{mergedClsPrefixRef:r,isHorizontalRef:t}=e;return()=>t.value?null:u("div",{class:`${r.value}-menu-divider`})}}),Ae=Object.assign(Object.assign({},pe),{tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function}),No=he(Ae),Ao=_({name:"MenuOption",props:Ae,setup(e){const r=ge(e),{NSubmenu:t,NMenu:v,NMenuOptionGroup:a}=r,{props:i,mergedClsPrefixRef:m,mergedCollapsedRef:f}=v,c=t?t.mergedDisabledRef:a?a.mergedDisabledRef:{value:!1},z=b(()=>c.value||e.disabled);function P(g){const{onClick:N}=e;N&&N(g)}function R(g){z.value||(v.doSelect(e.internalKey,e.tmNode.rawNode),P(g))}return{mergedClsPrefix:m,dropdownPlacement:r.dropdownPlacement,paddingLeft:r.paddingLeft,iconMarginRight:r.iconMarginRight,maxIconSize:r.maxIconSize,activeIconSize:r.activeIconSize,mergedTheme:v.mergedThemeRef,menuProps:i,dropdownEnabled:ae(()=>e.root&&f.value&&i.mode!=="horizontal"&&!z.value),selected:ae(()=>v.mergedValueRef.value===e.internalKey),mergedDisabled:z,handleClick:R}},render(){const{mergedClsPrefix:e,mergedTheme:r,tmNode:t,menuProps:{renderLabel:v,nodeProps:a}}=this,i=a==null?void 0:a(t.rawNode);return u("div",Object.assign({},i,{role:"menuitem",class:[`${e}-menu-item`,i==null?void 0:i.class]}),u(vo,{theme:r.peers.Tooltip,themeOverrides:r.peerOverrides.Tooltip,trigger:"hover",placement:this.dropdownPlacement,disabled:!this.dropdownEnabled||this.title===void 0,internalExtraClass:["menu-tooltip"]},{default:()=>v?v(t.rawNode):G(this.title),trigger:()=>u(Ne,{tmNode:t,clsPrefix:e,paddingLeft:this.paddingLeft,iconMarginRight:this.iconMarginRight,maxIconSize:this.maxIconSize,activeIconSize:this.activeIconSize,selected:this.selected,title:this.title,extra:this.extra,disabled:this.mergedDisabled,icon:this.icon,onClick:this.handleClick})}))}}),Te=Object.assign(Object.assign({},pe),{tmNode:{type:Object,required:!0},tmNodes:{type:Array,required:!0}}),To=he(Te),Ho=_({name:"MenuOptionGroup",props:Te,setup(e){const r=ge(e),{NSubmenu:t}=r,v=b(()=>t!=null&&t.mergedDisabledRef.value?!0:e.tmNode.disabled);X(fe,{paddingLeftRef:r.paddingLeft,mergedDisabledRef:v});const{mergedClsPrefixRef:a,props:i}=j(Z);return function(){const{value:m}=a,f=r.paddingLeft.value,{nodeProps:c}=i,z=c==null?void 0:c(e.tmNode.rawNode);return u("div",{class:`${m}-menu-item-group`,role:"group"},u("div",Object.assign({},z,{class:[`${m}-menu-item-group-title`,z==null?void 0:z.class],style:[(z==null?void 0:z.style)||"",f!==void 0?`padding-left: ${f}px;`:""]}),G(e.title),e.extra?u(Ue,null," ",G(e.extra)):null),u("div",null,e.tmNodes.map(P=>be(P,i))))}}});function se(e){return e.type==="divider"||e.type==="render"}function ko(e){return e.type==="divider"}function be(e,r){const{rawNode:t}=e,{show:v}=t;if(v===!1)return null;if(se(t))return ko(t)?u(Po,Object.assign({key:e.key},t.props)):null;const{labelField:a}=r,{key:i,level:m,isGroup:f}=e,c=Object.assign(Object.assign({},t),{title:t.title||t[a],extra:t.titleExtra||t.extra,key:i,internalKey:i,level:m,root:m===0,isGroup:f});return e.children?e.isGroup?u(Ho,te(c,To,{tmNode:e,tmNodes:e.children,key:i})):u(de,te(c,_o,{key:i,rawNodes:t[r.childrenField],tmNodes:e.children,tmNode:e})):u(Ao,te(c,No,{key:i,tmNode:e}))}const He=Object.assign(Object.assign({},pe),{rawNodes:{type:Array,default:()=>[]},tmNodes:{type:Array,default:()=>[]},tmNode:{type:Object,required:!0},disabled:Boolean,icon:Function,onClick:Function,domId:String,virtualChildActive:{type:Boolean,default:void 0},isEllipsisPlaceholder:Boolean}),_o=he(He),de=_({name:"Submenu",props:He,setup(e){const r=ge(e),{NMenu:t,NSubmenu:v}=r,{props:a,mergedCollapsedRef:i,mergedThemeRef:m}=t,f=b(()=>{const{disabled:g}=e;return v!=null&&v.mergedDisabledRef.value||a.disabled?!0:g}),c=$(!1);X(Pe,{paddingLeftRef:r.paddingLeft,mergedDisabledRef:f}),X(fe,null);function z(){const{onClick:g}=e;g&&g()}function P(){f.value||(i.value||t.toggleExpand(e.internalKey),z())}function R(g){c.value=g}return{menuProps:a,mergedTheme:m,doSelect:t.doSelect,inverted:t.invertedRef,isHorizontal:t.isHorizontalRef,mergedClsPrefix:t.mergedClsPrefixRef,maxIconSize:r.maxIconSize,activeIconSize:r.activeIconSize,iconMarginRight:r.iconMarginRight,dropdownPlacement:r.dropdownPlacement,dropdownShow:c,paddingLeft:r.paddingLeft,mergedDisabled:f,mergedValue:t.mergedValueRef,childActive:ae(()=>{var g;return(g=e.virtualChildActive)!==null&&g!==void 0?g:t.activePathRef.value.includes(e.internalKey)}),collapsed:b(()=>a.mode==="horizontal"?!1:i.value?!0:!t.mergedExpandedKeysRef.value.includes(e.internalKey)),dropdownEnabled:b(()=>!f.value&&(a.mode==="horizontal"||i.value)),handlePopoverShowChange:R,handleClick:P}},render(){var e;const{mergedClsPrefix:r,menuProps:{renderIcon:t,renderLabel:v}}=this,a=()=>{const{isHorizontal:m,paddingLeft:f,collapsed:c,mergedDisabled:z,maxIconSize:P,activeIconSize:R,title:g,childActive:N,icon:T,handleClick:A,menuProps:{nodeProps:k},dropdownShow:M,iconMarginRight:Y,tmNode:L,mergedClsPrefix:E,isEllipsisPlaceholder:S,extra:x}=this,C=k==null?void 0:k(L.rawNode);return u("div",Object.assign({},C,{class:[`${E}-menu-item`,C==null?void 0:C.class],role:"menuitem"}),u(Ne,{tmNode:L,paddingLeft:f,collapsed:c,disabled:z,iconMarginRight:Y,maxIconSize:P,activeIconSize:R,title:g,extra:x,showArrow:!m,childActive:N,clsPrefix:E,icon:T,hover:M,onClick:A,isEllipsisPlaceholder:S}))},i=()=>u(Ge,null,{default:()=>{const{tmNodes:m,collapsed:f}=this;return f?null:u("div",{class:`${r}-submenu-children`,role:"menu"},m.map(c=>be(c,this.menuProps)))}});return this.root?u(ho,Object.assign({size:"large",trigger:"hover"},(e=this.menuProps)===null||e===void 0?void 0:e.dropdownProps,{themeOverrides:this.mergedTheme.peerOverrides.Dropdown,theme:this.mergedTheme.peers.Dropdown,builtinThemeOverrides:{fontSizeLarge:"14px",optionIconSizeLarge:"18px"},value:this.mergedValue,disabled:!this.dropdownEnabled,placement:this.dropdownPlacement,keyField:this.menuProps.keyField,labelField:this.menuProps.labelField,childrenField:this.menuProps.childrenField,onUpdateShow:this.handlePopoverShowChange,options:this.rawNodes,onSelect:this.doSelect,inverted:this.inverted,renderIcon:t,renderLabel:v}),{default:()=>u("div",{class:`${r}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},a(),this.isHorizontal?null:i())}):u("div",{class:`${r}-submenu`,role:"menu","aria-expanded":!this.collapsed,id:this.domId},a(),i())}}),Eo=Object.assign(Object.assign({},q.props),{options:{type:Array,default:()=>[]},collapsed:{type:Boolean,default:void 0},collapsedWidth:{type:Number,default:48},iconSize:{type:Number,default:20},collapsedIconSize:{type:Number,default:24},rootIndent:Number,indent:{type:Number,default:32},labelField:{type:String,default:"label"},keyField:{type:String,default:"key"},childrenField:{type:String,default:"children"},disabledField:{type:String,default:"disabled"},defaultExpandAll:Boolean,defaultExpandedKeys:Array,expandedKeys:Array,value:[String,Number],defaultValue:{type:[String,Number],default:null},mode:{type:String,default:"vertical"},watchProps:{type:Array,default:void 0},disabled:Boolean,show:{type:Boolean,default:!0},inverted:Boolean,"onUpdate:expandedKeys":[Function,Array],onUpdateExpandedKeys:[Function,Array],onUpdateValue:[Function,Array],"onUpdate:value":[Function,Array],expandIcon:Function,renderIcon:Function,renderLabel:Function,renderExtra:Function,dropdownProps:Object,accordion:Boolean,nodeProps:Function,dropdownPlacement:{type:String,default:"bottom"},responsive:Boolean,items:Array,onOpenNamesChange:[Function,Array],onSelect:[Function,Array],onExpandedNamesChange:[Function,Array],expandedNames:Array,defaultExpandedNames:Array}),Oo=_({name:"Menu",inheritAttrs:!1,props:Eo,setup(e){const{mergedClsPrefixRef:r,inlineThemeDisabled:t}=ue(e),v=q("Menu","-menu",Ro,Xe,e,r),a=j(Re,null),i=b(()=>{var s;const{collapsed:p}=e;if(p!==void 0)return p;if(a){const{collapseModeRef:o,collapsedRef:h}=a;if(o.value==="width")return(s=h.value)!==null&&s!==void 0?s:!1}return!1}),m=b(()=>{const{keyField:s,childrenField:p,disabledField:o}=e;return ie(e.items||e.options,{getIgnored(h){return se(h)},getChildren(h){return h[p]},getDisabled(h){return h[o]},getKey(h){var w;return(w=h[s])!==null&&w!==void 0?w:h.name}})}),f=b(()=>new Set(m.value.treeNodes.map(s=>s.key))),{watchProps:c}=e,z=$(null);c!=null&&c.includes("defaultValue")?Ce(()=>{z.value=e.defaultValue}):z.value=e.defaultValue;const P=ee(e,"value"),R=ce(P,z),g=$([]),N=()=>{g.value=e.defaultExpandAll?m.value.getNonLeafKeys():e.defaultExpandedNames||e.defaultExpandedKeys||m.value.getPath(R.value,{includeSelf:!1}).keyPath};c!=null&&c.includes("defaultExpandedKeys")?Ce(N):N();const T=ao(e,["expandedNames","expandedKeys"]),A=ce(T,g),k=b(()=>m.value.treeNodes),M=b(()=>m.value.getPath(R.value).keyPath);X(Z,{props:e,mergedCollapsedRef:i,mergedThemeRef:v,mergedValueRef:R,mergedExpandedKeysRef:A,activePathRef:M,mergedClsPrefixRef:r,isHorizontalRef:b(()=>e.mode==="horizontal"),invertedRef:ee(e,"inverted"),doSelect:Y,toggleExpand:E});function Y(s,p){const{"onUpdate:value":o,onUpdateValue:h,onSelect:w}=e;h&&O(h,s,p),o&&O(o,s,p),w&&O(w,s,p),z.value=s}function L(s){const{"onUpdate:expandedKeys":p,onUpdateExpandedKeys:o,onExpandedNamesChange:h,onOpenNamesChange:w}=e;p&&O(p,s),o&&O(o,s),h&&O(h,s),w&&O(w,s),g.value=s}function E(s){const p=Array.from(A.value),o=p.findIndex(h=>h===s);if(~o)p.splice(o,1);else{if(e.accordion&&f.value.has(s)){const h=p.findIndex(w=>f.value.has(w));h>-1&&p.splice(h,1)}p.push(s)}L(p)}const S=s=>{const p=m.value.getPath(s??R.value,{includeSelf:!1}).keyPath;if(!p.length)return;const o=Array.from(A.value),h=new Set([...o,...p]);e.accordion&&f.value.forEach(w=>{h.has(w)&&!p.includes(w)&&h.delete(w)}),L(Array.from(h))},x=b(()=>{const{inverted:s}=e,{common:{cubicBezierEaseInOut:p},self:o}=v.value,{borderRadius:h,borderColorHorizontal:w,fontSize:$e,itemHeight:Le,dividerColor:Ke}=o,n={"--n-divider-color":Ke,"--n-bezier":p,"--n-font-size":$e,"--n-border-color-horizontal":w,"--n-border-radius":h,"--n-item-height":Le};return s?(n["--n-group-text-color"]=o.groupTextColorInverted,n["--n-color"]=o.colorInverted,n["--n-item-text-color"]=o.itemTextColorInverted,n["--n-item-text-color-hover"]=o.itemTextColorHoverInverted,n["--n-item-text-color-active"]=o.itemTextColorActiveInverted,n["--n-item-text-color-child-active"]=o.itemTextColorChildActiveInverted,n["--n-item-text-color-child-active-hover"]=o.itemTextColorChildActiveInverted,n["--n-item-text-color-active-hover"]=o.itemTextColorActiveHoverInverted,n["--n-item-icon-color"]=o.itemIconColorInverted,n["--n-item-icon-color-hover"]=o.itemIconColorHoverInverted,n["--n-item-icon-color-active"]=o.itemIconColorActiveInverted,n["--n-item-icon-color-active-hover"]=o.itemIconColorActiveHoverInverted,n["--n-item-icon-color-child-active"]=o.itemIconColorChildActiveInverted,n["--n-item-icon-color-child-active-hover"]=o.itemIconColorChildActiveHoverInverted,n["--n-item-icon-color-collapsed"]=o.itemIconColorCollapsedInverted,n["--n-item-text-color-horizontal"]=o.itemTextColorHorizontalInverted,n["--n-item-text-color-hover-horizontal"]=o.itemTextColorHoverHorizontalInverted,n["--n-item-text-color-active-horizontal"]=o.itemTextColorActiveHorizontalInverted,n["--n-item-text-color-child-active-horizontal"]=o.itemTextColorChildActiveHorizontalInverted,n["--n-item-text-color-child-active-hover-horizontal"]=o.itemTextColorChildActiveHoverHorizontalInverted,n["--n-item-text-color-active-hover-horizontal"]=o.itemTextColorActiveHoverHorizontalInverted,n["--n-item-icon-color-horizontal"]=o.itemIconColorHorizontalInverted,n["--n-item-icon-color-hover-horizontal"]=o.itemIconColorHoverHorizontalInverted,n["--n-item-icon-color-active-horizontal"]=o.itemIconColorActiveHorizontalInverted,n["--n-item-icon-color-active-hover-horizontal"]=o.itemIconColorActiveHoverHorizontalInverted,n["--n-item-icon-color-child-active-horizontal"]=o.itemIconColorChildActiveHorizontalInverted,n["--n-item-icon-color-child-active-hover-horizontal"]=o.itemIconColorChildActiveHoverHorizontalInverted,n["--n-arrow-color"]=o.arrowColorInverted,n["--n-arrow-color-hover"]=o.arrowColorHoverInverted,n["--n-arrow-color-active"]=o.arrowColorActiveInverted,n["--n-arrow-color-active-hover"]=o.arrowColorActiveHoverInverted,n["--n-arrow-color-child-active"]=o.arrowColorChildActiveInverted,n["--n-arrow-color-child-active-hover"]=o.arrowColorChildActiveHoverInverted,n["--n-item-color-hover"]=o.itemColorHoverInverted,n["--n-item-color-active"]=o.itemColorActiveInverted,n["--n-item-color-active-hover"]=o.itemColorActiveHoverInverted,n["--n-item-color-active-collapsed"]=o.itemColorActiveCollapsedInverted):(n["--n-group-text-color"]=o.groupTextColor,n["--n-color"]=o.color,n["--n-item-text-color"]=o.itemTextColor,n["--n-item-text-color-hover"]=o.itemTextColorHover,n["--n-item-text-color-active"]=o.itemTextColorActive,n["--n-item-text-color-child-active"]=o.itemTextColorChildActive,n["--n-item-text-color-child-active-hover"]=o.itemTextColorChildActiveHover,n["--n-item-text-color-active-hover"]=o.itemTextColorActiveHover,n["--n-item-icon-color"]=o.itemIconColor,n["--n-item-icon-color-hover"]=o.itemIconColorHover,n["--n-item-icon-color-active"]=o.itemIconColorActive,n["--n-item-icon-color-active-hover"]=o.itemIconColorActiveHover,n["--n-item-icon-color-child-active"]=o.itemIconColorChildActive,n["--n-item-icon-color-child-active-hover"]=o.itemIconColorChildActiveHover,n["--n-item-icon-color-collapsed"]=o.itemIconColorCollapsed,n["--n-item-text-color-horizontal"]=o.itemTextColorHorizontal,n["--n-item-text-color-hover-horizontal"]=o.itemTextColorHoverHorizontal,n["--n-item-text-color-active-horizontal"]=o.itemTextColorActiveHorizontal,n["--n-item-text-color-child-active-horizontal"]=o.itemTextColorChildActiveHorizontal,n["--n-item-text-color-child-active-hover-horizontal"]=o.itemTextColorChildActiveHoverHorizontal,n["--n-item-text-color-active-hover-horizontal"]=o.itemTextColorActiveHoverHorizontal,n["--n-item-icon-color-horizontal"]=o.itemIconColorHorizontal,n["--n-item-icon-color-hover-horizontal"]=o.itemIconColorHoverHorizontal,n["--n-item-icon-color-active-horizontal"]=o.itemIconColorActiveHorizontal,n["--n-item-icon-color-active-hover-horizontal"]=o.itemIconColorActiveHoverHorizontal,n["--n-item-icon-color-child-active-horizontal"]=o.itemIconColorChildActiveHorizontal,n["--n-item-icon-color-child-active-hover-horizontal"]=o.itemIconColorChildActiveHoverHorizontal,n["--n-arrow-color"]=o.arrowColor,n["--n-arrow-color-hover"]=o.arrowColorHover,n["--n-arrow-color-active"]=o.arrowColorActive,n["--n-arrow-color-active-hover"]=o.arrowColorActiveHover,n["--n-arrow-color-child-active"]=o.arrowColorChildActive,n["--n-arrow-color-child-active-hover"]=o.arrowColorChildActiveHover,n["--n-item-color-hover"]=o.itemColorHover,n["--n-item-color-active"]=o.itemColorActive,n["--n-item-color-active-hover"]=o.itemColorActiveHover,n["--n-item-color-active-collapsed"]=o.itemColorActiveCollapsed),n}),C=t?ve("menu",b(()=>e.inverted?"a":"b"),x,e):void 0,K=Ye(),F=$(null),oe=$(null);let H=!0;const xe=()=>{var s;H?H=!1:(s=F.value)===null||s===void 0||s.sync({showAllItemsBeforeCalculate:!0})};function ke(){return document.getElementById(K)}const Q=$(-1);function _e(s){Q.value=e.options.length-s}function Ee(s){s||(Q.value=-1)}const Oe=b(()=>{const s=Q.value;return{children:s===-1?[]:e.options.slice(s)}}),Me=b(()=>{const{childrenField:s,disabledField:p,keyField:o}=e;return ie([Oe.value],{getIgnored(h){return se(h)},getChildren(h){return h[s]},getDisabled(h){return h[p]},getKey(h){var w;return(w=h[o])!==null&&w!==void 0?w:h.name}})}),Be=b(()=>ie([{}]).treeNodes[0]);function Fe(){var s;if(Q.value===-1)return u(de,{root:!0,level:0,key:"__ellpisisGroupPlaceholder__",internalKey:"__ellpisisGroupPlaceholder__",title:"···",tmNode:Be.value,domId:K,isEllipsisPlaceholder:!0});const p=Me.value.treeNodes[0],o=M.value,h=!!(!((s=p.children)===null||s===void 0)&&s.some(w=>o.includes(w.key)));return u(de,{level:0,root:!0,key:"__ellpisisGroup__",internalKey:"__ellpisisGroup__",title:"···",virtualChildActive:h,tmNode:p,domId:K,rawNodes:p.rawNode.children||[],tmNodes:p.children||[],isEllipsisPlaceholder:!0})}return{mergedClsPrefix:r,controlledExpandedKeys:T,uncontrolledExpanededKeys:g,mergedExpandedKeys:A,uncontrolledValue:z,mergedValue:R,activePath:M,tmNodes:k,mergedTheme:v,mergedCollapsed:i,cssVars:t?void 0:x,themeClass:C==null?void 0:C.themeClass,overflowRef:F,counterRef:oe,updateCounter:()=>{},onResize:xe,onUpdateOverflow:Ee,onUpdateCount:_e,renderCounter:Fe,getCounter:ke,onRender:C==null?void 0:C.onRender,showOption:S,deriveResponsiveState:xe}},render(){const{mergedClsPrefix:e,mode:r,themeClass:t,onRender:v}=this;v==null||v();const a=()=>this.tmNodes.map(c=>be(c,this.$props)),m=r==="horizontal"&&this.responsive,f=()=>u("div",We(this.$attrs,{role:r==="horizontal"?"menubar":"menu",class:[`${e}-menu`,t,`${e}-menu--${r}`,m&&`${e}-menu--responsive`,this.mergedCollapsed&&`${e}-menu--collapsed`],style:this.cssVars}),m?u(mo,{ref:"overflowRef",onUpdateOverflow:this.onUpdateOverflow,getCounter:this.getCounter,onUpdateCount:this.onUpdateCount,updateCounter:this.updateCounter,style:{width:"100%",display:"flex",overflow:"hidden"}},{default:a,counter:this.renderCounter}):a());return m?u(qe,{onResize:this.onResize},{default:f}):f()}}),Mo={class:"right"},Bo={class:"who"},Fo=_({__name:"Shell",setup(e){const r=Ze(),t=co(),v=io(),a=lo(),i=go(),m=[{label:"账号与直播间",key:"accounts"},{label:"房管",key:"moderation"},{label:"弹幕姬",key:"danmaku"},{label:"自定义弹幕姬",key:"custom"},{label:"统计",key:"stats"},{label:"日志",key:"logs"},{label:"管理",key:"admin"}];Qe(()=>void t.refresh());function f(z){if(!a.hasRoute(z)){i.info("这个页面还没做");return}a.push({name:z})}function c(){r.logout().catch(()=>{}).finally(()=>a.push("/login"))}return(z,P)=>(eo(),Je(B(ne),{"has-sider":"",position:"absolute"},{default:U(()=>[V(B(So),{bordered:"",width:180,"content-style":"padding-top: 12px"},{default:U(()=>[V(B(Oo),{value:String(B(v).name),options:m,"onUpdate:value":f},null,8,["value"])]),_:1}),V(B(ne),null,{default:U(()=>[V(B(Co),{bordered:"",class:"header"},{default:U(()=>{var R;return[P[1]||(P[1]=re("div",{class:"left"},null,-1)),re("div",Mo,[re("span",Bo,oo((R=B(r).user)==null?void 0:R.username),1),V(B(to),{text:"",size:"small",onClick:c},{default:U(()=>[...P[0]||(P[0]=[ro(" 退出 ",-1)])]),_:1})])]}),_:1}),V(B(ne),{"content-style":"padding: 16px"},{default:U(()=>[V(B(no))]),_:1})]),_:1})]),_:1}))}}),qo=fo(Fo,[["__scopeId","data-v-00b7b155"]]);export{qo as default};
