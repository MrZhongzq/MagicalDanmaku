import{a6 as I,a7 as R,a8 as j,a9 as C,c as x,a as m,b as L,e as O,h as f,S as z,u as E,f as g,Q as P,x as $,r as y,n as S,p as w}from"./index-DtJCLJeh.js";function F(h){const{baseColor:e,textColor2:r,bodyColor:a,cardColor:i,dividerColor:t,actionColor:d,scrollbarColor:b,scrollbarColorHover:u,invertedColor:s}=h;return{textColor:r,textColorInverted:"#FFF",color:a,colorEmbedded:d,headerColor:i,headerColorInverted:s,footerColor:d,footerColorInverted:s,headerBorderColor:t,headerBorderColorInverted:s,footerBorderColor:t,footerBorderColorInverted:s,siderBorderColor:t,siderBorderColorInverted:s,siderColor:i,siderColorInverted:s,siderToggleButtonBorder:`1px solid ${t}`,siderToggleButtonColor:e,siderToggleButtonIconColor:r,siderToggleButtonIconColorInverted:r,siderToggleBarColor:C(a,b),siderToggleBarColorHover:C(a,u),__invertScrollbar:"true"}}const N=I({name:"Layout",common:j,peers:{Scrollbar:R},self:F}),M=x("n-layout-sider"),_={type:String,default:"static"},H=m("layout",`
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
`,[m("layout-scroll-container",`
 overflow-x: hidden;
 box-sizing: border-box;
 height: 100%;
 `),L("absolute-positioned",`
 position: absolute;
 left: 0;
 right: 0;
 top: 0;
 bottom: 0;
 `)]),K={embedded:Boolean,position:_,nativeScrollbar:{type:Boolean,default:!0},scrollbarProps:Object,onScroll:Function,contentClass:String,contentStyle:{type:[String,Object],default:""},hasSider:Boolean,siderPlacement:{type:String,default:"left"}},V=x("n-layout");function k(h){return O({name:"Layout",props:Object.assign(Object.assign({},g.props),K),setup(e){const r=y(null),a=y(null),{mergedClsPrefixRef:i,inlineThemeDisabled:t}=E(e),d=g("Layout","-layout",H,N,e,i);function b(o,l){if(e.nativeScrollbar){const{value:n}=r;n&&(l===void 0?n.scrollTo(o):n.scrollTo(o,l))}else{const{value:n}=a;n&&n.scrollTo(o,l)}}w(V,e);let u=0,s=0;const p=o=>{var l;const n=o.target;u=n.scrollLeft,s=n.scrollTop,(l=e.onScroll)===null||l===void 0||l.call(e,o)};P(()=>{if(e.nativeScrollbar){const o=r.value;o&&(o.scrollTop=s,o.scrollLeft=u)}});const T={display:"flex",flexWrap:"nowrap",width:"100%",flexDirection:"row"},B={scrollTo:b},v=S(()=>{const{common:{cubicBezierEaseInOut:o},self:l}=d.value;return{"--n-bezier":o,"--n-color":e.embedded?l.colorEmbedded:l.color,"--n-text-color":l.textColor}}),c=t?$("layout",S(()=>e.embedded?"e":""),v,e):void 0;return Object.assign({mergedClsPrefix:i,scrollableElRef:r,scrollbarInstRef:a,hasSiderStyle:T,mergedTheme:d,handleNativeElScroll:p,cssVars:t?void 0:v,themeClass:c==null?void 0:c.themeClass,onRender:c==null?void 0:c.onRender},B)},render(){var e;const{mergedClsPrefix:r,hasSider:a}=this;(e=this.onRender)===null||e===void 0||e.call(this);const i=a?this.hasSiderStyle:void 0,t=[this.themeClass,h,`${r}-layout`,`${r}-layout--${this.position}-positioned`];return f("div",{class:t,style:this.cssVars},this.nativeScrollbar?f("div",{ref:"scrollableElRef",class:[`${r}-layout-scroll-container`,this.contentClass],style:[this.contentStyle,i],onScroll:this.handleNativeElScroll},this.$slots):f(z,Object.assign({},this.scrollbarProps,{onScroll:this.onScroll,ref:"scrollbarInstRef",theme:this.mergedTheme.peers.Scrollbar,themeOverrides:this.mergedTheme.peerOverrides.Scrollbar,contentClass:this.contentClass,contentStyle:[this.contentStyle,i]}),this.$slots))}})}const Q=k(!1);export{Q as N,V as a,M as b,N as l,_ as p};
